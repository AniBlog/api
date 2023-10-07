package main

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"os"
)

type BlogPost struct {
	Id    int64  `json:"id"`
	Title string `json:"title"`
}

func GetPostById(c *gin.Context) {
	//id := c.Param("id")
	post := BlogPost{
		Id:    1,
		Title: "Test",
	}
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
