package main

import (
      elastic "gopkg.in/olivere/elastic.v3"

      "encoding/json"
      "fmt"
      "net/http"
      "reflect"
      "regexp"
      "time"

      "github.com/dgrijalva/jwt-go"
)
//
const (
      TYPE_USER = "user"
)
//正则表达式判断username是否符号规范 不能有大写字母 ^:匹配最开始 $:匹配到末尾 +:一个或者更多 *:0个或更多 []:
var (
      usernamePattern = regexp.MustCompile(`^[a-z0-9_]+$`).MatchString
)

type User struct {
      Username string `json:"username"`
      Password string `json:"password"`
      Age int `json:"age"`
      Gender string `json:"gender"`
}
// checkUser checks whether user is valid
func checkUser(username, password string) bool {
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		   fmt.Printf("ES is not setup %v\n", err)
		   panic(err)
	}

	// Search with a term query
	termQuery := elastic.NewTermQuery("username", username)
	queryResult, err := es_client.Search().
		   Index(INDEX).//database 每个应用用自己的index 互不影响
		   Query(termQuery).
		   Pretty(true).
		   Do()
	if err != nil {
		   fmt.Printf("ES query failed %v\n", err)
		   return false
	}

	var tyu User
	//为什么要用循环 返回类型是slice 理论上只会循环一次
	for _, item := range queryResult.Each(reflect.TypeOf(tyu)) {
		   u := item.(User)
		   return u.Password == password && u.Username == username
	}
	// If no user exist, return false.
	return false
}
// add user adds a new user
func addUser(user User) bool {
	es_client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false))
	if err != nil {
		fmt.Printf("ES is not setup %v\n", err)
		return false
	}
    //先要搜索一下用户
	termQuery := elastic.NewTermQuery("username", user.Username)
	queryResult, err := es_client.Search().
		Index(INDEX).
		Query(termQuery).
		Pretty(true).
		Do()
	if err != nil {
		fmt.Printf("ES query failed %v\n", err)
		return false;
	}
    //
	if queryResult.TotalHits() > 0 {
		fmt.Printf("User %s already exists, cannot create duplicate user.\n", user.Username)
		return false
	}

	_, err = es_client.Index().
		Index(INDEX).
		Type(TYPE_USER).
		Id(user.Username).
		BodyJson(user).
		Refresh(true).
		Do()
	if err != nil {
		fmt.Printf("ES save user failed %v\n", err)
		return false
	}

	return true
}

// If signup is successful, a new session is created.
func signupHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one signup request")

	decoder := json.NewDecoder(r.Body)
	var u User
	if err := decoder.Decode(&u); err != nil {
		   panic(err)
		   return
	}

	if u.Username != "" && u.Password != "" && usernamePattern(u.Username) {
		   if addUser(u) {
				  fmt.Println("User added successfully.")
				  w.Write([]byte("User added successfully."))
		   } else {
				  fmt.Println("Failed to add a new user.")
				  http.Error(w, "Failed to add a new user", http.StatusInternalServerError)
		   }
	} else {
		   fmt.Println("Empty password or username.")
		   http.Error(w, "Empty password or username", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// If login is successful, a new token is created.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one login request")

	decoder := json.NewDecoder(r.Body)
	var u User
	if err := decoder.Decode(&u); err != nil {
		   panic(err)
		   return
	}

	if checkUser(u.Username, u.Password) {
		   token := jwt.New(jwt.SigningMethodHS256)
		   claims := token.Claims.(jwt.MapClaims)
		   /* Set token claims */
		   claims["username"] = u.Username
		   claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

		   /* Sign the token with our secret */
		   tokenString, _ := token.SignedString(mySigningKey)

		   /* Finally, write the token to the browser window */
		   w.Write([]byte(tokenString))
	} else {
		   fmt.Println("Invalid password or username.")
		   http.Error(w, "Invalid password or username", http.StatusForbidden)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

