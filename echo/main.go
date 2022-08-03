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

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/labstack/echo/v4"
)

var conn *pgxpool.Pool
var stops []Stop

func main() {
	// LOGGER
	LOG_FILE := "kapi.log"
    logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
    if err != nil {
        log.Panic(err)
    }
    defer logFile.Close()
    log.SetOutput(logFile)
    log.Printf("Started")

	// POSTGRES
    conn, err = pgxpool.Connect(context.Background(), "host=localhost user=kabina password=kaboot dbname=kabina port=5432 sslmode=disable TimeZone=Europe/Oslo")
	if err != nil {
		log.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// global STOPS
    stops, err = getStopsSrvc();
    if err != nil {
        log.Printf("Unable to get stops: %v\n", err)
		os.Exit(1)
    }
    log.Printf("Read %d stops", len(stops));

	// API
	e := echo.New()

	e.GET("/cabs/:id", getCab) // curl -u cab1:cab1 http://localhost:8080/cabs/1916
    e.PUT("/cabs", putCab) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":2, "location": 123, "status":"FREE", "name":"A2"}' http://localhost:8080/cabs
	e.PUT("/cabs/", putCab) 
    e.PUT("/legs", putLeg) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":17081, "status":"STARTED"}' http://localhost:8080/legs
	e.PUT("/legs/", putLeg) 
    e.GET("/routes", getRoute) // curl -u cab2:cab2 http://localhost:8080/routes
	e.GET("/routes/", getRoute)
    e.PUT("/routes", putRoute) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":9724, "status":"ASSIGNED"}' http://localhost:8080/routes
	e.PUT("/routes/", putRoute)
    e.GET("/stops", getStops) // curl u cab2:cab2 http://localhost:8080/stops
	e.GET("/stops/", getStops)
    e.GET("/orders/:id", getOrder) // curl -u cab2:cab2 http://localhost:8080/orders/51150
    e.PUT("/orders", putOrder) // curl -H "Content-type: application/json" -H "Accept: application/json"  -X PUT -u cab1:cab1 -d '{ "id":51150, "status":"ASSIGNED"}' http://localhost:8080/orders
	e.PUT("/orders/", putOrder)
    e.POST("/orders", postOrder) //curl -H "Content-type: application/json" -H "Accept: application/json"  -X POST -u "cust28:cust28" -d '{"fromStand":4082, "toStand":4083, "maxWait":10, "maxLoss":90, "shared": true}' http://localhost:8080/orders
	e.POST("/orders/", postOrder)
   
	e.Logger.Fatal(e.Start(":8080"))
}

// ----------------- UTILITIES ---------------

func getUserId(c echo.Context) int {
    user, _, _ := c.Request().BasicAuth()
    if strings.HasPrefix(user, "cab") || strings.HasPrefix(user, "adm") {
      	id, err := strconv.Atoi(user[3:]) 
    	if err == nil { return id } else { return -1 }
	} else if strings.HasPrefix(user, "cust") {
		id, err := strconv.Atoi(user[4:]) 
	  	if err == nil { return id } else { return -1 }
  	} 
	return -1
}

func getParam(c echo.Context, name string) int64 {
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
func getCab(c echo.Context) error {
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
	return nil;
}

func putCab(c echo.Context) error {
    // buf, _ := ioutil.ReadAll(c.Request.Body)
    // rdr1 := ioutil.NopCloser(bytes.NewBuffer(buf))
    // fmt.Println(readBody(rdr1)) 

    user := getUserId(c)
    var cab Cab
    if err := c.Bind(&cab); err != nil {
        log.Printf("PUT cab failed, usr_id=%d", user)
        return nil
    }
    log.Printf("PUT cab_id=%d, status=%s location=%d usr_id=%d", 
                cab.Id, cab.Status, cab.Location, user)
    c.JSON(http.StatusOK, putCabSrvc(cab))
	return nil
}
/*
func readBody(reader io.Reader) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(reader)

	s := buf.String()
	return s
}
*/
// ORDER
func getOrder(c echo.Context) error {
    user := getUserId(c)
    order_id  := getParam(c, "id")
    log.Printf("GET order_id=%d, usr_id=%d", order_id, user)
    if order_id == -1 || user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong order_id or user"})
    }
    ret, err := getOrderSrvc(order_id)
    if err != nil {
        c.JSON(http.StatusNotFound, map[string]interface{}{"message": err})
    } else {
        c.JSON(http.StatusOK, ret)
    }
	return nil
}

func putOrder(c echo.Context) error {
    user := getUserId(c)
    var order Order
    if err := c.Bind(&order); err != nil {
        log.Printf("PUT order failed, usr_id=%d", user)
        return nil
    }
    log.Printf("PUT order_id=%d, status=%s usr_id=%d", order.Id, order.Status, user)
    c.JSON(http.StatusOK, putOrderSrvc(order))
	return nil
}

func postOrder(c echo.Context) error {
    user := getUserId(c)
    var order Order
    if err := c.Bind(&order); err != nil {
        log.Printf("POST order failed, usr_id=%d", user)
        return nil
	}
	
    log.Printf("POST order from=%d to=%d usr_id=%d", order.FromStand, order.ToStand, user)
    ret, err := postOrderSrvc(order, user)
    if err != nil {
		log.Printf("POST order failed: %v\n", err)
		return nil
	}
    c.JSON(http.StatusOK, ret)
	return nil
}

// LEG
func putLeg(c echo.Context) error {
    user := getUserId(c)
    var leg Leg
    if err := c.Bind(&leg); err != nil {
        log.Printf("PUT leg failed, usr_id=%d", user)
        return nil
    }
    log.Printf("PUT leg_id=%d, status=%s usr_id=%d", leg.Id, leg.Status, user)
    c.JSON(http.StatusOK, putLegSrvc(leg))
	return nil
}

// ROUTE
func getRoute(c echo.Context) error {
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
	return nil
}

func putRoute(c echo.Context) error {
    user := getUserId(c)
    var route Route
    if err := c.Bind(&route); err != nil {
        log.Printf("PUT route failed, usr_id=%d", user)
        return nil
    }
    log.Printf("PUT route_id=%d, status=%s usr_id=%d", route.Id, route.Status, user)
    c.JSON(http.StatusOK, putRouteSrvc(route))
	return nil
}

// STOP
func getStops(c echo.Context) error {
    user := getUserId(c)
    log.Printf("GET stops usr_id=%d", user)
    if user == -1 {
        c.JSON(http.StatusForbidden, map[string]interface{}{"message": "wrong user"})
    }
    c.JSON(http.StatusOK, stops)
	return nil
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
        orderStatus int
        location int
        cab sql.NullInt64
        cab_id int64 = -1
        o Order
    )
	err := conn.QueryRow(context.Background(), 
        "SELECT distance, eta, from_stand, to_stand, in_pool, shared, max_wait, max_loss, status, cab_id FROM taxi_order WHERE id=$1", 
        id).Scan(&o.Distance, &o.Eta, &o.FromStand, &o.ToStand, &o.InPool, &o.Shared, &o.MaxWait,&o.MaxLoss, &orderStatus, &cab)
    
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
            o.Cab = Cab { Id: cab_id, Location: location, Name: name.String, Status: cabStatusStr[status]}
        }
    }
    o.Status = orderStatusStr[orderStatus]
    o.Id = id
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
    if o.FromStand == o.ToStand {
        log.Printf("cust_id=%d is a joker", cust_id)
        return o, errors.New("Refused")
    }
    dist := GetDistance(o.FromStand, o.ToStand)
    t := time.Now()
    timeNow := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d",
                        t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
    sqlStatement := fmt.Sprintf(
        "INSERT INTO taxi_order (from_stand, to_stand, max_loss, max_wait, shared, in_pool, eta," +
			"status, received, distance, customer_id) VALUES (%d,%d,%d,%d,%t,false,%d,%d,'%s',%d,%d) RETURNING (id)", 
        o.FromStand, o.ToStand, o.MaxLoss, o.MaxWait, o.Shared, -1, orderStatusInt["ASSIGNED"],
        timeNow, dist, cust_id)
    var id int64 = -1;
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
        "SELECT id FROM route WHERE cab_id=$1 AND status=1 ORDER BY id LIMIT 1", //status=1: ASSIGNED
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
            FromStand: values[1].(int32),
	        ToStand: values[2].(int32),
	        Place: values[3].(int32),
            Status: legStatusStr[values[4].(int32)],
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


