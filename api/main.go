package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4"
)

var conn *pgx.Conn
var stops []Stop

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
		log.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
    stops, err = getStopsSrvc();
    if err != nil {
        log.Printf("Unable to get stops: %v\n", err)
		os.Exit(1)
    }
    log.Printf("Read %d stops", len(stops));
    
    router := gin.Default()
    router.GET("/cabs/:id", basicAuth, getCab) // curl -u cab1:cab1 http://localhost:8080/cabs/1916
    router.PUT("/cabs", basicAuth, putCab) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":2, "location": 123, "status":"FREE", "name":"A2"}' http://localhost:8080/cabs
    router.PUT("/legs", basicAuth, putLeg)
    router.GET("/routes", basicAuth, getRoute)
    router.PUT("/routes", basicAuth, putRoute)
    router.GET("/stops", basicAuth, getStops)
    router.GET("/orders/:id", basicAuth, getOrder)
    router.PUT("/orders", basicAuth, putOrder)
    router.POST("/orders", basicAuth, postOrder) //curl -X POST -u "cust28:cust28" -d '{"id":-1, "fromStand":4082, "toSTand":"4083", "maxWait":10, "maxLoss":90, "shared": true}' https://localhost:8080/orders
   
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

func getParam(c *gin.Context, name string) int64 {
    id, err := strconv.Atoi(c.Param(name))
    if err == nil { return int64(id) } else { return -1 }
}

// -------- DISTANCE SERVICE --------------
func Dist(lat1 float64, lon1 float64, lat2 float64, lon2 float64) float64 {
	var theta = lon1 - lon2;
	var dist = math.Sin(deg2rad(lat1)) * math.Sin(deg2rad(lat2)) + math.Cos(deg2rad(lat1)) * math.Cos(deg2rad(lat2)) * math.Cos(deg2rad(theta));
	dist = math.Acos(dist);
	dist = rad2deg(dist);
	dist = dist * 60 * 1.1515;
	dist = dist * 1.609344;
	return (dist);
}

func deg2rad(deg float64) float64 {
	return (deg * math.Pi / 180.0);
}

func rad2deg(rad float64) float64 {
	return (rad * 180.0 / math.Pi);
}

func GetDistance(from_id int, to_id int) int {
	var from = -1
	var to = -1
	for x :=0; x<len(stops); x++ {
		if stops[x].Id == int64(from_id) {
			from = x
			break
		}
	}
	for x :=0; x<len(stops); x++ {
		if stops[x].Id == int64(to_id) {
			to = x
			break
		}
	}
	if from == -1 || to == -1 {
		fmt.Printf("from %d or to %d ID not found in stops", from_id, to_id)
		return -1
	}
	return int (Dist(stops[from].Latitude, stops[from].Longitude, 
					 stops[to].Latitude, stops[to].Longitude));
}

// ------------- CONTROLERS ---------------

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
    c.JSON(http.StatusOK, ret)
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
    c.JSON(http.StatusOK, putCabSrvc(cab))
}

// ORDER
func getOrder(c *gin.Context) {
    user := getUserId(c)
    order_id  := getParam(c, "id")
    log.Printf("GET order_id=%d, usr_id=%d", order_id, user)
    if order_id == -1 || user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong order_id or user"})
    }
    ret, err := getOrderSrvc(order_id)
    if err != nil {
        c.JSON(http.StatusNotFound, map[string]interface{}{"message": err})
    }
    c.JSON(http.StatusOK, ret)
}

func putOrder(c *gin.Context) {
    user := getUserId(c)
    var order Order
    if err := c.BindJSON(&order); err != nil {
        log.Printf("PUT order failed, usr_id=%d", user)
        return
    }
    log.Printf("PUT order_id=%d, status=%s usr_id=%d", order.Id, order.Status, user)
    c.JSON(http.StatusOK, putOrderSrvc(order))
}

func postOrder(c *gin.Context) {
    user := getUserId(c)
    var order Order
    if err := c.BindJSON(&order); err != nil {
        log.Printf("POST order failed, usr_id=%d", user)
        return
    }
    log.Printf("POST order_id=%d, status=%s usr_id=%d", order.Id, order.Status, user)
    ret, err := postOrderSrvc(order, user)
    if err != nil {
		log.Printf("POST order failed: %v\n", err)
		return;
	}
    c.JSON(http.StatusOK, ret)
}

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
    c.JSON(http.StatusOK, ret)
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
func getStops(c *gin.Context) {
    user := getUserId(c)
    log.Printf("GET stops usr_id=%d", user)
    if user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong user"})
    }
    c.JSON(http.StatusOK, stops)
}

// ----------- SERVICE / REPO --------------

// CAB
func getCabSrvc(id int64) (Cab, error) {
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
log.Printf("!! %s", sqlStatement)
    conn.QueryRow(context.Background(), sqlStatement)
    return cab;
}

func postCabSrvc(cab Cab) (Cab, error) {
    sqlStatement := fmt.Sprintf(
        "INSERT INTO cab (name, status, location) VALUES ('%s',%d,%d) RETURNING (id)", 
        cab.Name, cabStatusInt[cab.Status], cab.Location)
    var id int64 = -1;
    err := conn.QueryRow(context.Background(), sqlStatement).Scan(&id)
    cab.Id = id
    return cab, err;
}

// ORDER
func getOrderSrvc(id int64) (Order, error) {
    var ( 
        name sql.NullString
        status int
        location int
        cab sql.NullInt64
        cab_id int64 = -1
        o Order
    )
	err := conn.QueryRow(context.Background(), 
        "SELECT distance, eta, from_stand, to_stand, in_pool, max_wait, max_loss, status, cab_id FROM taxi_order WHERE id=$1", 
        id).Scan(&o.Distance, &o.Eta, &o.From, &o.To, &o.InPool, &o.MaxWait,&o.MaxLoss, &o.Status, &cab)
	if err != nil {
		log.Printf("Select order failed: %v\n", err)
		return o, err;
	}
    if cab.Valid {
        cab_id = cab.Int64
        err = conn.QueryRow(context.Background(), 
            "SELECT location, status, name FROM cab WHERE id=$1", cab_id).Scan(&location, &status, &name)
        if err != nil {
            log.Printf("SELECT cab failed for order_id=%d cab_id=%d", id, cab_id);
        } else {
            o.Cab = Cab { Id: id, Location: location, Name: name.String, Status: cabStatusStr[status]}
        }
    }
    return o, err
}

func putOrderSrvc(o Order) Order {
    sqlStatement := fmt.Sprintf(
        "UPDATE taxi_order SET status=%d WHERE id=%d", 
            orderStatusInt[o.Status], o.Id)
    conn.QueryRow(context.Background(), sqlStatement)
    return o;
}

func postOrderSrvc(o Order, cust_id int) (Order, error) {
    if o.From == o.To {
        log.Printf("cust_id=%d is a joker", cust_id)
        return o, errors.New("Refused")
    }
    dist := GetDistance(o.From, o.To)

    sqlStatement := fmt.Sprintf(
        "INSERT INTO taxi_order (from_stand, to_stand, max_loss, max_wait, shared, in_pool, eta," +
			"status, received, distance, cust_id) VALUES (%d,%d,%d,%d,%t,false,%d) RETURNING (id)", 
        o.From, o.To, o.MaxLoss, o.MaxWait, o.Shared, false, -1, orderStatusInt["ASSIGNED"],
        time.Now().String(), dist, cust_id)
    var id int = -1;
    err := conn.QueryRow(context.Background(), sqlStatement).Scan(&id)
    o.Id = id
    return o, err;
}

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
            Id: values[0].(int64),
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
func getStopsSrvc() ([]Stop, error) {
    var stops []Stop
	rows, err := conn.Query(context.Background(), 
                "SELECT id, no, name, type, bearing, latitude, longitude FROM stop")
	if err != nil {
		log.Printf("Select stops failed: %v\n", err)
		return stops, err;
	}
    for rows.Next() {
        values, err := rows.Values()
        if err != nil {
            log.Printf("error while iterating dataset")
            return stops, err;
        }
        var stopType string
        if values[3] == nil { stopType = "" } else { stopType = values[3].(string) } 
        stop := Stop {
            Id: 	values[0].(int64),
            No: 	values[1].(string),
            Name: 	values[2].(string),
            Type: 	stopType,
            Bearing: values[4].(int32),
            Latitude: values[5].(float64),
            Longitude:values[6].(float64),
        }
        stops = append(stops, stop)
    }
    return stops, err
}

