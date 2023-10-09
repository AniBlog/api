package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"os"
	"time"
)

type Site struct {
	Id   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Post struct {
	Site          Site   `json:"site"`
	Id            int64  `json:"id"`
	Title         string `json:"title"`
	Url           string `json:"url"`
	Summary       string `json:"summary"`
	DatePublished string `json:"date"`
	ImageUrl      string `json:"image"`
}

func GetPostById(c *gin.Context) {
	post := Post{
		Site: Site{
			Id:   1,
			Name: "Bateszi Anime Blog",
			Type: "Aniblog",
		},
		Id:            1,
		Title:         "The End of Vinland Saga: An Inevitable Tragedy",
		Url:           "https://bateszi.me/2020/01/03/the-end-of-vinland-saga-an-inevitable-tragedy/",
		Summary:       "Pellentesque elit ullamcorper dignissim cras tincidunt lobortis feugiat vivamus at augue eget arcu dictum varius duis at consectetur lorem donec massa sapien faucibus et molestie",
		DatePublished: time.Now().Format(time.RFC3339),
		ImageUrl:      "https://cdn.aniblogtracker.com/live/20201231/1609434169.122.2989.jpg",
	}
	//id := c.Param("id")
	//post := BlogPost{
	//	Id:    1,
	//	Title: "Test",
	//}
	c.IndentedJSON(http.StatusOK, post)
}

func main() {
	dbConnectionString := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=utf8mb4,utf8",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASS"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_NAME"),
	)
	db, err := sql.Open("mysql", dbConnectionString)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected!")
	router := gin.Default()
	apiV1 := router.Group("/v1")
	apiV1.GET("/post/:id", GetPostById)
	err = router.Run("localhost:1234")
	if err != nil {
		log.Fatal(err)
	}
}
