package main

import (
	"fmt"
	"encoding/json"
	"net/http"
	"log"
	"strconv"
	elastic "gopkg.in/olivere/elastic.v3"
	"github.com/pborman/uuid"
	"reflect"
)

type Location struct{
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
}
// Go 里面大写的User 对应json里面小写的user
//``标记raw string
type Post struct{
	User string `json:"user"`
	Message string `json:"message"`
	Location Location `json:"location"`
}
const (
	INDEX = "around"//
	TYPE = "post"
	DISTANCE = "200km"
	// Needs to update
	//PROJECT_ID = "around-xxx"
	//BT_INSTANCE = "around-post"
	// Needs to update this URL if you deploy it to cloud.
	ES_URL = "http://35.239.4.96:9200/"

)
func main() {
	// Create a client
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Use the IndexExists service to check if a specified index exists.
	exists, err := client.IndexExists(INDEX).Do()
	if err != nil {
		panic(err)
	}
	if !exists {
		// Create a new index.
		mapping := `{
			"mappings":{
				"post":{
					"properties":{
						"location":{
							"type":"geo_point"
						}
					}
				}
			}
		}`
		_, err := client.CreateIndex(INDEX).Body(mapping).Do()
		if err != nil {
			// Handle error
			panic(err)
		}
	}

	fmt.Println("started-service")
	http.HandleFunc("/post",handlerPost)//第一个参数是endpoint 第二个参数是endpoint用哪个函数来保存
	http.HandleFunc("/search",handlerSearch)
	log.Fatal(http.ListenAndServe(":8080",nil))
}
//大写 相当于public 可以外部来引用
//小写      private 外部无法引用
// struct 类似class


//函数里修改r 外面也会变化
// r是用户提交的 r.body就是引用文字的部分
func handlerPost(w http.ResponseWriter, r *http.Request){
	fmt.Println("Received one post request.")
	decoder := json.NewDecoder(r.Body)
	var p Post
	//if 写两个statement 第一个用来初始化一些变量(可以单独写) 第二个做真正的判断
	//&p 传P的地址 可以直接编辑地址里面的值
	if err :=decoder.Decode(&p); err!=nil{
		panic(err)
		return
	}
	id := uuid.New()
	// Save to ES.
	saveToES(&p, id)
}
// Save a post to ElasticSearch
func saveToES(p *Post, id string) {
	// Create a client
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
		return
	}

	// Save it to index
	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE).
		Id(id).
		BodyJson(p).
		Refresh(true).
		Do()
	if err != nil {
		panic(err)
		return
	}

	fmt.Printf("Post is saved to Index: %s\n", p.Message)
}

func handlerSearch(w http.ResponseWriter, r *http.Request){
	fmt.Println("Received one request for search.")
	//ParseFloat 有两个返回值  _改为err的话可以后面用了 go定义不使用的话不可以  _就是不考虑
	lat,_ := strconv.ParseFloat(r.URL.Query().Get("lat"),64)
	lon,_ := strconv.ParseFloat(r.URL.Query().Get("lon"),64)

	 // range is optional 
	 ran := DISTANCE 
	 if val := r.URL.Query().Get("range"); val != "" { 
		ran = val + "km" 
	 }

	
	 fmt.Printf( "Search received: %f %f %s\n", lat, lon, ran)

      // Create a client
	  client, err := elastic.NewClient(elastic.SetURL(ES_URL), 
	  elastic.SetSniff(false))//不需要回调函数来记录状态
      if err != nil {
             panic(err)
             return
      }

      // Define geo distance query as specified in
      // https://www.elastic.co/guide/en/elasticsearch/reference/5.2/query-dsl-geo-distance-query.html
      q := elastic.NewGeoDistanceQuery("location")
      q = q.Distance(ran).Lat(lat).Lon(lon)

      // Some delay may range from seconds to minutes. So if you don't get enough results. Try it later.
      searchResult, err := client.Search().
             Index(INDEX).
             Query(q).
             Pretty(true).
             Do()
      if err != nil {
             // Handle error
             panic(err)
      }

      // searchResult is of type SearchResult and returns hits, suggestions,
      // and all kinds of other information from Elasticsearch.
      fmt.Printf("Query took %d milliseconds\n", searchResult.TookInMillis)
      // TotalHits is another convenience function that works even when something goes wrong.
      fmt.Printf("Found a total of %d post\n", searchResult.TotalHits())

      // Each is a convenience function that iterates over hits in a search result.
      // It makes sure you don't need to check for nil values in the response.
      // However, it ignores errors in serialization.
      var typ Post
	  var ps []Post
	  //interface相当于object 找到post类型
      for _, item := range searchResult.Each(reflect.TypeOf(typ)) { // instance of
             p := item.(Post) // p = (Post) item
             fmt.Printf("Post by %s: %s at lat %v and lon %v\n", p.User, p.Message, p.Location.Lat, p.Location.Lon)
             // TODO(student homework): Perform filtering based on keywords such as web spam etc.
			 
			 
			 ps = append(ps, p)

      }
      js, err := json.Marshal(ps)
      if err != nil {
             panic(err)
             return
      }

      w.Header().Set("Content-Type", "application/json")
      w.Header().Set("Access-Control-Allow-Origin", "*")
      w.Write(js)

}
