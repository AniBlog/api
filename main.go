package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ApiResponsePosts struct {
	Posts      []Post     `json:"posts"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	NumFound int `json:"ttl"`
	Start    int `json:"start"`
	Rows     int `json:"rows"`
}

type Post struct {
	PostId          string    `json:"post_id"`
	PostTitle       string    `json:"post_title"`
	PostLink        string    `json:"post_link"`
	PostDescription string    `json:"post_description"`
	PostPubDate     time.Time `json:"post_pub_date"`
	PostImage       string    `json:"post_image"`
	PostTags        []string  `json:"post_tags"`
	PostMedia       []string  `json:"post_media"`
	trendingScore   float64
	Site            Site `json:"site"`
}

type SolrHandler struct {
	Address string
}

type SolrQuery struct {
	Q     string `default:"*"`
	Rows  int    `default:"50"`
	Start int    `default:"0"`
	FQ    string
	Sort  string
}

type SolrResponse struct {
	ResponseHeader struct {
		Status int `json:"status"`
		QTime  int `json:"QTime"`
		Params struct {
			Q      string `json:"q"`
			Indent string `json:"indent"`
			QOp    string `json:"q.op"`
			Fq     string `json:"fq"`
		} `json:"params"`
	} `json:"responseHeader"`
	Response struct {
		NumFound      int  `json:"numFound"`
		Start         int  `json:"start"`
		NumFoundExact bool `json:"numFoundExact"`
		Docs          []struct {
			PostTitle           string    `json:"post_title"`
			PostPubDateRangeUtc time.Time `json:"post_pub_date_range_utc"`
			SiteId              int       `json:"site_id"`
			PostLink            string    `json:"post_link"`
			PostDescription     string    `json:"post_description"`
			SiteType            string    `json:"site_type"`
			Id                  string    `json:"id"`
			SiteName            string    `json:"site_name"`
			ViewCount           int       `json:"view_count"`
			PostImage           string    `json:"post_image"`
			PostTags            []string  `json:"post_tags"`
			PostMedia           []string  `json:"post_media"`
			Version             int64     `json:"_version_"`
			PostPubDateSorter   time.Time `json:"post_pub_date_sorter"`
		} `json:"docs"`
	} `json:"response"`
}

type Site struct {
	SiteId   int    `json:"site_id"`
	SiteName string `json:"site_name"`
	SiteType string `json:"site_type"`
}

func (receiver *SolrHandler) request(query *SolrQuery) (SolrResponse, error) {
	var solrResponseInstance SolrResponse
	baseURL, err := url.Parse(fmt.Sprintf("%s/solr/rss/select", receiver.Address))
	if err != nil {
		return solrResponseInstance, err
	}

	params := url.Values{}
	if query.Rows == 0 {
		query.Rows = 50
	}
	if query.Q != "" {
		params.Add("q", query.Q)
	} else {
		params.Add("q", "*")
	}
	if query.FQ != "" {
		params.Add("fq", query.FQ)
	}
	if query.Sort != "" {
		params.Add("sort", query.Sort)
	}
	params.Add("rows", fmt.Sprintf("%d", query.Rows))
	params.Add("start", fmt.Sprintf("%d", query.Start))
	baseURL.RawQuery = params.Encode()
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return solrResponseInstance, err
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(&solrResponseInstance)
	return solrResponseInstance, nil
}

// The decay factor determines how rapidly a post's score decays over time. If you want to favor posts published in the last 7 days, you would choose a decay factor that makes the post's score significantly decrease after 7 days.
// To find an appropriate decay factor, you can set up a simple equation. Let's call the decay factor \( d \). If you want the score to be half of its original value after 7 days, then:
//
// \[ d^7 = 0.5 \]
//
// Solving for \( d \) would give:
//
// \[ d = \sqrt[7]{0.5} \]
//
// To calculate this in Go:
//
// ```go
// decayFactor := math.Pow(0.5, 1.0/7.0)
// ```
//
// This would make the score of a post roughly half after 7 days.
//
// If you want a more aggressive decay (e.g., you want the score to be a third of its original value after 7 days), you'd use:
//
// \[ d = \sqrt[7]{0.33} \]
//
// Adjust the target fraction (0.5 in the first example, 0.33 in the second) to control how aggressively you want the score to decay over a 7-day period.
func (receiver *ApiResponsePosts) algo(trendingPostViews map[string]int64) {
	decayFactor := math.Pow(0.33, 1.0/7.0)
	for i := range receiver.Posts {
		postAgeInDays := float64(time.Now().Unix()-receiver.Posts[i].PostPubDate.Unix()) / (24 * 60 * 60)
		multiplier := math.Pow(decayFactor, postAgeInDays)
		receiver.Posts[i].trendingScore = float64(trendingPostViews[receiver.Posts[i].PostId]) * multiplier
	}
	sort.Slice(receiver.Posts, func(i, j int) bool {
		return receiver.Posts[j].trendingScore < receiver.Posts[i].trendingScore
	})
	//fixme get rid of posts with a trending score of 0
}

func ApiV1LatestPosts(c *gin.Context) {
	start, hasStart := c.GetQuery("start")
	solr, _ := c.MustGet("solr").(SolrHandler)
	var solrQuery = SolrQuery{
		Sort: "post_pub_date_sorter desc",
	}
	if hasStart {
		start, _ := strconv.Atoi(start)
		solrQuery.Start = start
	}
	solrResponse, _ := solr.request(&solrQuery)
	apiResponse := NewApiResponse(solrResponse)
	c.IndentedJSON(http.StatusOK, apiResponse)
}

func ApiV1SearchPosts(c *gin.Context) {
	var solrQuery SolrQuery

	query, hasQuery := c.GetQuery("query")
	if hasQuery {
		solrQuery.Q = query
	}

	start, hasStart := c.GetQuery("start")
	if hasStart {
		start, _ := strconv.Atoi(start)
		solrQuery.Start = start
	}

	sorter, hasSorter := c.GetQuery("sorter")
	if hasSorter {
		solrQuery.Sort = sorter
	}

	solr, _ := c.MustGet("solr").(SolrHandler)

	solrResponse, err := solr.request(&solrQuery)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}

	apiResponse := NewApiResponse(solrResponse)
	c.IndentedJSON(http.StatusOK, apiResponse)
}

func ApiV1TrendingPosts(c *gin.Context) {
	db, _ := c.MustGet("db").(*sql.DB)
	PruneStaleViews(db)
	solr, _ := c.MustGet("solr").(SolrHandler)
	rows, _ := db.Query("SELECT fk_post_id, COUNT(*) as views, pub_date " +
		"FROM post_views, posts " +
		"WHERE post_views.fk_post_id = posts.pk_post_id " +
		"AND post_views.created >= NOW() - INTERVAL 7 DAY " +
		"AND posts.pub_date >= NOW() - INTERVAL 14 DAY " +
		"GROUP BY fk_post_id " +
		"ORDER BY views DESC " +
		"LIMIT 250")
	defer rows.Close()
	trendingPostIds := make([]string, 0)
	trendingPostViews := make(map[string]int64)
	for rows.Next() {
		var postId string
		var postViews int64
		var publishedDate string
		rows.Scan(&postId, &postViews, &publishedDate)
		trendingPostIds = append(trendingPostIds, postId)
		trendingPostViews[postId] = postViews
	}
	var solrQuery = SolrQuery{
		FQ:   fmt.Sprintf("id:(%s)", strings.Join(trendingPostIds, " ")),
		Rows: 250,
	}
	solrResponse, _ := solr.request(&solrQuery)
	apiResponse := NewApiResponse(solrResponse)
	apiResponse.algo(trendingPostViews)
	c.IndentedJSON(http.StatusOK, apiResponse)
}

func NewApiResponse(solrResponse SolrResponse) ApiResponsePosts {
	var apiResponse ApiResponsePosts
	for _, searchDoc := range solrResponse.Response.Docs {
		apiResponse.Posts = append(apiResponse.Posts, Post{
			PostId:          searchDoc.Id,
			PostTitle:       searchDoc.PostTitle,
			PostLink:        searchDoc.PostLink,
			PostDescription: searchDoc.PostDescription,
			PostPubDate:     searchDoc.PostPubDateRangeUtc,
			PostImage:       searchDoc.PostImage,
			PostTags:        searchDoc.PostTags,
			PostMedia:       searchDoc.PostMedia,
			Site: Site{
				SiteId:   searchDoc.SiteId,
				SiteName: searchDoc.SiteName,
				SiteType: searchDoc.SiteType,
			},
		})
	}
	apiResponse.Pagination.NumFound = solrResponse.Response.NumFound
	apiResponse.Pagination.Start = solrResponse.Response.Start
	apiResponse.Pagination.Rows = len(solrResponse.Response.Docs)
	return apiResponse
}

func PruneStaleViews(db *sql.DB) {
	db.Exec("DELETE FROM post_views WHERE created < NOW() - INTERVAL 7 DAY")
}

func main() {
	dbConnectionString := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4,utf8",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
	)
	db, _ := sql.Open("mysql", dbConnectionString)
	_ = db.Ping()
	fmt.Println("Connected!")
	router := gin.Default()
	apiV1 := router.Group("/v1")
	apiV1.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Set("solr", SolrHandler{Address: os.Getenv("SOLR_ADDRESS")})
		c.Next()
	})
	apiV1.GET("/posts/trending", ApiV1TrendingPosts)
	apiV1.GET("/posts/latest", ApiV1LatestPosts)
	apiV1.GET("/posts/search", ApiV1SearchPosts)
	apiAddress := fmt.Sprintf(
		":%s",
		os.Getenv("API_PORT"),
	)
	_ = router.Run(apiAddress)
}
