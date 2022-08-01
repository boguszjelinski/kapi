package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4"
)

var conn *pgx.Conn

func main() {
    LOG_FILE := "kapi.log"
    logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
        log.Panic(err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)
    log.Printf("Started")
    conn, err = pgx.Connect(context.Background(), "host=localhost user=kabina password=kaboot dbname=kabina port=5432 sslmode=disable TimeZone=Europe/Oslo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

    router := gin.Default()
    router.GET("/cabs/:id", basicAuth, getCab) // http://localhost:8080/albums
    router.Run("localhost:8080")

}

// /cabs/1
func getCab(c *gin.Context) {
    user := getUserId(c)
    cab_id  := getParam(c, "id")
    log.Printf("GET cab_id={%d} usr_id={%d}", cab_id, user)
    if cab_id == -1 || user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong cab_id or user"})
    }
    ret, err := getCabSrvc(cab_id)
    if err != nil {
        c.JSON(http.StatusNotFound, map[string]interface{}{"message": err})
    }
    c.IndentedJSON(http.StatusOK, ret)
}

func basicAuth(c *gin.Context) {
	user, password, hasAuth := c.Request.BasicAuth()
	if hasAuth && ((strings.HasPrefix(user, "cab") && strings.HasPrefix(password, "cab")) ||
        (strings.HasPrefix(user, "cust") && strings.HasPrefix(password, "cust")) ||
        (strings.HasPrefix(user, "adm") && strings.HasPrefix(password, "adm"))) {		
            // do nothing
	} else {
		c.Abort()
		c.Writer.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
		return
	}
}

func getUserId(c *gin.Context) int {
    user, _, _ := c.Request.BasicAuth()
    if !strings.HasPrefix(user, "cab") {
        return -1
    }
    id, err := strconv.Atoi(user[3:]) 
    if err == nil { return id } else { return -1 }
}

func getParam(c *gin.Context, name string) int {
    id, err := strconv.Atoi(c.Param(name))
    if err == nil { return id } else { return -1 }
}

func getCabSrvc(id int) (Cab, error) {
    var name sql.NullString
	var status CabStatus
    var location int
    var cab Cab
	err := conn.QueryRow(context.Background(), 
        "select location, status, name FROM cab WHERE id=$1", id).Scan(&location, &status, &name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		return cab, err;
	}
    cab = Cab{ Id: id, Location: location, Name: name.String, Status: status};
    return cab, err
}

func putCabSrvc(cab Cab) (Cab, error) {
    sqlStatement := fmt.Sprintf(
        "INSERT INTO cab (name, status, location) VALUES ('%s',%d,%d) RETURNING (id)", 
        cab.Name, cab.Status, cab.Location)
    var id int = -1;
    err := conn.QueryRow(context.Background(), sqlStatement).Scan(&id)
    cab.Id = id
    return cab, err;
}
