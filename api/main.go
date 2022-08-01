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
	"github.com/jackc/pgx/v4/pgxpool"
)

var conn *pgxpool.Pool

func main() {
    LOG_FILE := "kapi.log"
    logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
        log.Panic(err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)
    log.Printf("Started")
    conn, err = pgxpool.Connect(context.Background(), "host=localhost user=kabina password=kaboot dbname=kabina port=5432 sslmode=disable TimeZone=Europe/Oslo")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

    router := gin.Default()
    router.GET("/cabs/:id", basicAuth, getCab) // http://localhost:8080/albums
    router.PUT("/cabs", basicAuth, putCab) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":2, "location": 123, "status":"FREE", "name":"A2"}' http://localhost:8080/cabs
    router.PUT("/legs", basicAuth, putLeg)
    router.GET("/routes", basicAuth, getRoute)
    router.PUT("/routes", basicAuth, putRoute)
   //router.GET("/stops", basicAuth, getStops)
    //router.GET("/orders/:id", basicAuth, getOrder)
    //router.PUT("/orders", basicAuth, putOrder)
    //router.POST("/orders", basicAuth, postOrder)
    router.Run("localhost:8080")
}

// ----------------- UTILITIES ---------------
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

// --------------- CONTROLERS ----------------
//
// CAB
func getCab(c *gin.Context) {
    user := getUserId(c)
    cab_id  := getParam(c, "id")
    log.Printf("GET cab_id=%d, usr_id=%d", cab_id, user)
    if cab_id == -1 || user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong cab_id or user"})
    }
    ret, err := getCabSrvc(cab_id)
    if err != nil {
        c.JSON(http.StatusNotFound, map[string]interface{}{"message": err})
    }
    c.IndentedJSON(http.StatusOK, ret)
}

func putCab(c *gin.Context) {
    user := getUserId(c)
    var cab Cab
    if err := c.BindJSON(&cab); err != nil {
        log.Printf("PUT cab failed, usr_id=%d", user)
        return
    }
    log.Printf("PUT cab_id=%d, status=%s location=%d usr_id=%d", 
                cab.Id, cab.Status, cab.Location, user)
    c.IndentedJSON(http.StatusOK, putCabSrvc(cab))
}


// ORDER


// LEG
func putLeg(c *gin.Context) {
    user := getUserId(c)
    var leg Leg
    if err := c.BindJSON(&leg); err != nil {
        log.Printf("PUT leg failed, usr_id=%d", user)
        return
    }
    log.Printf("PUT leg_id=%d, status=%s usr_id=%d", leg.Id, leg.Status, user)
    c.IndentedJSON(http.StatusOK, putLegSrvc(leg))
}

// ROUTE
func getRoute(c *gin.Context) {
    user := getUserId(c)
    log.Printf("GET route usr_id=%d", user)
    if user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong user"})
    }
    ret, err := getRouteSrvc(user)
    if err != nil {
        c.JSON(http.StatusNotFound, map[string]interface{}{"message": err})
    }
    c.IndentedJSON(http.StatusOK, ret)
}

func putRoute(c *gin.Context) {
    user := getUserId(c)
    var route Route
    if err := c.BindJSON(&route); err != nil {
        log.Printf("PUT route failed, usr_id=%d", user)
        return
    }
    log.Printf("PUT route_id=%d, status=%s usr_id=%d", route.Id, route.Status, user)
    c.IndentedJSON(http.StatusOK, putRouteSrvc(route))
}
// STOP


// -------------- SERVICE / REPO ----------------
//
// CAB
func getCabSrvc(id int) (Cab, error) {
    var name sql.NullString
	var status int
    var location int
    var cab Cab
	err := conn.QueryRow(context.Background(), 
        "SELECT location, status, name FROM cab WHERE id=$1", id).Scan(&location, &status, &name)
	if err != nil {
		log.Printf("Select cab failed: %v\n", err)
		return cab, err;
	}
    /*
    err = conn.QueryRow(context.Background(), 
        "SELECT id,distance,eta,from_stand,to_stand,in_pool,max_wait,max_loss,status FROM taxi_order WHERE cab_id=$1", id).Scan(...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Select cab failed: %v\n", err)
		return cab, err;
	}
    */
    cab = Cab{ Id: id, Location: location, Name: name.String, Status: cabStatusStr[status]};
    return cab, err
}

func putCabSrvc(cab Cab) Cab {
    sqlStatement := fmt.Sprintf(
        "UPDATE cab SET status=%d, location=%d WHERE id=%d", 
            cabStatusInt[cab.Status], cab.Location, cab.Id)
    conn.QueryRow(context.Background(), sqlStatement)
    return cab;
}

func postCabSrvc(cab Cab) (Cab, error) {
    sqlStatement := fmt.Sprintf(
        "INSERT INTO cab (name, status, location) VALUES ('%s',%d,%d) RETURNING (id)", 
        cab.Name, cabStatusInt[cab.Status], cab.Location)
    var id int = -1;
    err := conn.QueryRow(context.Background(), sqlStatement).Scan(&id)
    cab.Id = id
    return cab, err;
}


// ORDER


// LEG
func putLegSrvc(leg Leg) Leg {
    sqlStatement := fmt.Sprintf(
        "UPDATE leg SET status=%d WHERE id=%d", legStatusInt[leg.Status], leg.Id)
    conn.QueryRow(context.Background(), sqlStatement)
    return leg;
}

// ROUTE
func getRouteSrvc(cab_id int) (Route, error) {
    var id int
    var route Route
	err := conn.QueryRow(context.Background(), 
        "SELECT id FROM route WHERE cab_id=$1 AND status=1 ORDER BY id LIMIT 1", //1: ASSIGNED
        cab_id).Scan(&id)
	if err != nil {
		log.Printf("Select route failed: %v\n", err)
		return route, err;
	}
    rows, err := conn.Query(context.Background(), 
        "SELECT id, from_stand, to_stand, place, status FROM leg WHERE route_id=$1", id)
    if err != nil {
        log.Printf("Select legs for route failed: %v\n", err)
        return route, err;
    }
    
    var legs []Leg
    for rows.Next() {
        values, err := rows.Values()
        if err != nil {
            log.Printf("error while iterating dataset")
            return route, err;
        }
        leg := Leg {
            Id: values[0].(int),
            FromStand: values[1].(int),
	        ToStand: values[2].(int),
	        Place: values[3].(int),
            Status: values[4].(string),
        }
        legs = append(legs, leg)
    }
    route = Route{
        Id: id,
        Status: "ASSIGNED",
        Legs: legs,
    }
    return route, err
}

func putRouteSrvc(route Route) Route {
    sqlStatement := fmt.Sprintf(
        "UPDATE route SET status=%d WHERE id=%d", legStatusInt[route.Status], route.Id)
    conn.QueryRow(context.Background(), sqlStatement)
    return route;
}

// STOP

