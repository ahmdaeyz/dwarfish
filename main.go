package main

/*
implementing a url shortening service
database :nonsql solution -> mongodb
document model :
{
	longURL: `string`
	shortURL: `string`
}
urls are kept for 24 hours
GET	dwarfish.herokuapp.com/s/{shortURL} will be redirected to -> LONG URL
POST dwarfish.herokuapp.com/l body has longURL in json (might add other features so this is safe)
*/
import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"strconv"
	"time"
)
type postUrl struct{
	LongURL string `json:"long_url"`
	Life int `json:"life"`
}
func main(){
	mongoURI:="mongodb+srv://ahmdaeyz:ahmd1234@cluster0-i9ke0.mongodb.net/test?retryWrites=true&w=majority"
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err!=nil{
		log.Fatal(err)
	}
	collection := client.Database("dwarfish").Collection("urls")
	//go func(){
	//		for {
	//			collection.Aggregate(ctx,bson.A{bson.M{"$project":bson.M{"keep":bson.M{"$cond":bson.M{"if":bson.M{"$eq":bson.A{"$expires","$currentDate"}},"then":false,"else":true}}}}})
	//			time.Sleep(1*time.Second)
	//		}
	//	}()
	r:= gin.Default()
	r.GET("/s/:short", func(c *gin.Context) {
		shortURL:=c.Param("short")
		var result bson.M
		ctx, _ = context.WithTimeout(context.Background(), 10*time.Second)
		lookup:=collection.FindOne(ctx,bson.M{"short_url":shortURL})
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
	r.POST("/l", func(i *gin.Context) {
		var postURL postUrl
		var token string
		var expires time.Time
		if err:= i.ShouldBindJSON(&postURL);err!=nil{
			i.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if postURL.Life<24&&postURL.Life!=0{
			i.JSON(http.StatusBadRequest,gin.H{"error":"url life should be more than 24 hours"})
			return
		}
		for{
			token = randstr.String(5)
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			count,err:=collection.CountDocuments(ctx,bson.M{"short_url":token})
			if err!=nil{
				i.JSON(http.StatusBadGateway,gin.H{"error":errors.New("database error")})
			}
			if count==1{
				break
			}
		}
		if postURL.Life!=0{
			duration, _:=time.ParseDuration(strconv.Itoa(postURL.Life)+"h")
			expires=time.Now().Add(duration)
		}
		_,err=collection.InsertOne(ctx,bson.M{"long_url":postURL.LongURL,"short_url":token,"expires":expires ,"keep":true})
		if err!=nil{
			i.JSON(502,gin.H{"error":err})
			return
		}
		i.JSON(200,gin.H{
			"long_url":postURL.LongURL,
			"short_url":token,
			"expires": expires.Format(time.ANSIC),
		})
	})
	log.Fatal(r.Run())
}