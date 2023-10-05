package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
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
	router := gin.Default()
	apiV1 := router.Group("/v1")
	apiV1.GET("/post/:id", GetPostById)
	router.Run("localhost:1234")
}
