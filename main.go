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
	"strings"
	"time"
)

type TrendingPosts struct {
	Posts []TrendingPost
}

type TrendingPost struct {
	PostId        string
	Views         int64
	PublishedDate time.Time
	TrendingScore float64
}

type ApiResponsePosts struct {
	Posts []Post `json:"posts"`
}

type Post struct {
	PostId          string    `json:"post_id"`
	PostTitle       string    `json:"post_title"`
	PostLink        string    `json:"post_link"`
	PostDescription string    `json:"post_description"`
	PostPubDate     time.Time `json:"post_pub_date"`
	PostImage       string    `json:"post_image"`
	PostTags        []string  `json:"post_tags"`
	trendingScore   float64
	Site            Site `json:"site"`
}

type Site struct {
	SiteName string `json:"site_name"`
	SiteType string `json:"site_type"`
}

func (receiver *ApiResponsePosts) UnmarshalJSON(data []byte) error {
	var SolrSearchResults struct {
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
				Version             int64     `json:"_version_"`
				PostPubDateSorter   time.Time `json:"post_pub_date_sorter"`
			} `json:"docs"`
		} `json:"response"`
	}
	json.Unmarshal(data, &SolrSearchResults)
	for _, searchDoc := range SolrSearchResults.Response.Docs {
		receiver.Posts = append(receiver.Posts, Post{
			PostId:          searchDoc.Id,
			PostTitle:       searchDoc.PostTitle,
			PostLink:        searchDoc.PostLink,
			PostDescription: searchDoc.PostDescription,
			PostPubDate:     searchDoc.PostPubDateRangeUtc,
			PostImage:       searchDoc.PostImage,
			PostTags:        searchDoc.PostTags,
			Site: Site{
				SiteName: searchDoc.SiteName,
				SiteType: searchDoc.SiteType,
			},
		})
	}
	return nil
}

func (receiver *ApiResponsePosts) algo(trendingPostViews map[string]int64) {
	const decayFactor = 0.985
	for i := range receiver.Posts {
		postAgeInDays := float64(time.Now().Unix()-receiver.Posts[i].PostPubDate.Unix()) / (24 * 60 * 60)
		multiplier := math.Pow(decayFactor, postAgeInDays)
		receiver.Posts[i].trendingScore = float64(trendingPostViews[receiver.Posts[i].PostId]) * multiplier
	}
	sort.Slice(receiver.Posts, func(i, j int) bool {
		return receiver.Posts[j].trendingScore < receiver.Posts[i].trendingScore
	})
}

func ApiV1TrendingPosts(c *gin.Context) {
	db, _ := c.MustGet("db").(*sql.DB)
	solrAddress, _ := c.MustGet("solr").(string)
	rows, _ := db.Query("SELECT post_views.fk_post_id, COUNT(*) as Views, posts.pub_date " +
		"FROM post_views, posts " +
		"WHERE post_views.fk_post_id = posts.pk_post_id " +
		"AND post_views.created >= NOW() - INTERVAL 2 DAY " +
		"AND posts.visible = 1 " +
		"GROUP BY post_views.fk_post_id " +
		"ORDER BY Views DESC " +
		"LIMIT 5")
	defer rows.Close()
	trendingPostIds := make([]string, 0)
	trendingPostViews := make(map[string]int64, 0)
	for rows.Next() {
		var trendingPost TrendingPost
		var publishedDate string
		rows.Scan(&trendingPost.PostId, &trendingPost.Views, &publishedDate)
		trendingPost.PublishedDate, _ = time.Parse("2006-01-02 15:04:05", publishedDate)
		trendingPostIds = append(trendingPostIds, trendingPost.PostId)
		trendingPostViews[trendingPost.PostId] = trendingPost.Views
	}
	baseURL, _ := url.Parse(fmt.Sprintf("%s/solr/rss/select", solrAddress))
	params := url.Values{}
	params.Add("q", "*")
	params.Add("rows", "100")
	params.Add("start", "0")
	params.Add("fq", fmt.Sprintf("id:(%s)", strings.Join(trendingPostIds, " ")))
	baseURL.RawQuery = params.Encode()
	resp, _ := http.Get(baseURL.String())
	defer resp.Body.Close()
	var apiResponse ApiResponsePosts
	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(&apiResponse)
	apiResponse.algo(trendingPostViews)
	c.IndentedJSON(http.StatusOK, apiResponse)
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
		c.Set("solr", os.Getenv("SOLR_ADDRESS"))
		c.Next()
	})
	apiV1.GET("/posts/trending", ApiV1TrendingPosts)
	apiAddress := fmt.Sprintf(
		"%s:%s",
		os.Getenv("API_HOST"),
		os.Getenv("API_PORT"),
	)
	_ = router.Run(apiAddress)
}
