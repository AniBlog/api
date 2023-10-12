package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"
)

type TrendingPosts struct {
	Posts []TrendingPost `json:"posts"`
}

type TrendingPost struct {
	PostId        int64 `json:"post_id"`
	views         int64
	publishedDate time.Time
	TrendingScore float64
}

type Post struct {
	PostId          int64
	PostTitle       string
	PostLink        string
	PostDescription string
	PostPubDate     string
	PostImage       string
	PostTags        []string
	Site            Site
}

type Site struct {
	SiteName string
	SiteType string
}

func (receiver *TrendingPosts) algo() {
	const decayFactor = 0.9986

	for i := range receiver.Posts {
		postAgeInDays := float64(time.Now().Unix()-receiver.Posts[i].publishedDate.Unix()) / (24 * 60 * 60)
		multiplier := math.Pow(decayFactor, postAgeInDays)
		receiver.Posts[i].TrendingScore = float64(receiver.Posts[i].views) * multiplier
	}

	sort.Slice(receiver.Posts, func(i, j int) bool {
		return receiver.Posts[j].TrendingScore < receiver.Posts[i].TrendingScore
	})
}

func ApiV1TrendingPosts(c *gin.Context) {
	db, _ := c.MustGet("db").(*sql.DB)
	rows, _ := db.Query("SELECT post_views.fk_post_id, COUNT(*) as views, posts.pub_date " +
		"FROM post_views, posts " +
		"WHERE post_views.fk_post_id = posts.pk_post_id " +
		"AND post_views.created >= NOW() - INTERVAL 1 DAY " +
		"GROUP BY post_views.fk_post_id " +
		"ORDER BY views DESC " +
		"LIMIT 250")
	defer rows.Close()
	trendingPosts := TrendingPosts{Posts: make([]TrendingPost, 0)}
	for rows.Next() {
		var trendingPost TrendingPost
		var publishedDate string
		rows.Scan(&trendingPost.PostId, &trendingPost.views, &publishedDate)
		trendingPost.publishedDate, _ = time.Parse("2006-01-02 15:04:05", publishedDate)
		trendingPosts.Posts = append(trendingPosts.Posts, trendingPost)
	}
	trendingPosts.algo()
	baseURL, _ := url.Parse("http://localhost:8983/solr/rss/select")
	params := url.Values{}
	params.Add("q", "value1")
	params.Add("rows", "value2")
	params.Add("start", "value2")
	params.Add("fq", "value2")
	baseURL.RawQuery = params.Encode()
	resp, _ := http.Get(baseURL.String())
	defer resp.Body.Close()
	c.IndentedJSON(http.StatusOK, trendingPosts)
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
