package main
import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/paked/configure"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)
type postUrl struct{
	LongURL string `json:"long_url"`
}
var(
	conf = configure.New()
	mongoURI = conf.String("mongo_uri","mongo uri","MongoDB URI")
	client *mongo.Client
	collection *mongo.Collection
	err error
)
func determineListenAddress() (string, error) {
	port := os.Getenv("PORT")
	if port == "" {
		return "", fmt.Errorf("$PORT not set")
	}
	return ":" + port, nil
}
func init(){
	conf.Use(configure.NewEnvironment())
	conf.Use(configure.NewFlag())
	conf.Use(configure.NewJSONFromFile("./config.json"))
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err!=nil{
		log.Fatal(err)
	}
	collection = client.Database("dwarfish").Collection("urls")
}
func main(){
	gin.SetMode(gin.ReleaseMode)
	r:= gin.Default()
	r.GET("/s/:short", func(c *gin.Context) {
		shortURL:=c.Param("short")
		var result bson.M
		ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
		lookup:=collection.FindOneAndUpdate(ctx,bson.M{"short_url":shortURL},bson.M{"$inc":bson.M{"views":1}})
		err=lookup.Decode(&result)
		if err==mongo.ErrNoDocuments{
			err=c.AbortWithError(404,errors.New("url doesn't exist"))
			if err!=nil{
				log.Println(err)
			}
			return
		}
		c.Redirect(http.StatusPermanentRedirect,fmt.Sprintf("%v", result["long_url"]))
	})
	r.GET("/i/:short", func(i *gin.Context) {
		shortURL:= i.Param("short")
		var result bson.M
		ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
		lookup:=collection.FindOne(ctx,bson.M{"short_url":shortURL})
		err=lookup.Decode(&result)
		if err==mongo.ErrNoDocuments{
			i.AbortWithStatusJSON(404,gin.H{"error":errors.New("url doesn't exist").Error()})
			return
		}
		views,_:=strconv.Atoi(fmt.Sprintf("%d",result["long_url"]))
		i.JSON(200,gin.H{
			"long_url":fmt.Sprintf("%v",result["long_url"]),
			"short_url":fmt.Sprintf("%v",result["short_url"]),
			"views":views,
		})
	})
	r.POST("/l", func(i *gin.Context) {
		var postURL postUrl
		var token string
		if err:= i.ShouldBindJSON(&postURL);err!=nil{
			i.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		for{
			token = randstr.String(5)
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			count,err:=collection.CountDocuments(ctx,bson.M{"short_url":token})
			if err!=nil{
				i.JSON(http.StatusBadGateway,gin.H{"error":errors.New("database error")})
				break
			}
			if count==0{
				break
			}
			log.Println(token ,"duplicate")
		}
		ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
		_,err=collection.InsertOne(ctx,bson.M{"long_url":postURL.LongURL,"short_url":token,"views":0})
		if err!=nil{
			i.JSON(502,gin.H{"error":err.Error()})
			return
		}
		i.JSON(200,gin.H{
			"long_url":postURL.LongURL,
			"short_url":token,
			"views":0,
		})
	})
	listeningAt, err := determineListenAddress()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(r.Run(listeningAt))
}