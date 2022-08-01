package main

// -------------- CAB ---------------
type Cab struct {
	Id       int `json:"id"`
	Location int `json:"location"`
	Status   string `json:"status"`
	Name     string `json:"name"`
	//orders	 []Order
}

var cabStatusStr = map[int]string{
	0: "ASSIGNED",
	1: "FREE",
	2: "CHARGING",
}

var cabStatusInt = map[string]int{
	"ASSIGNED": 0,
	"FREE": 1,
	"CHARGING": 2,
}

// ------------- STOP -------------
type Stop struct {
	Id 		int
	No 		string
    Name 	string
    Type 	string
    Bearing string
    Latitude 	float64
    Longitude 	float64
}

// ----------- ROUTE ------------
type Route struct {
	Id int
    Status string
    Legs []Leg
}

// ------------ LEG -------------
type Leg struct {
	Id int
	FromStand int
	ToStand int 
	Place int
    Status string
}

var legStatusStr = map[int]string{
	0: "PLANNED",  // proposed by Pool
	1: "ASSIGNED", // not confirmed, initial status
	2: "ACCEPTED", // plan accepted by customer, waiting for the cab
	3: "REJECTED", // proposal rejected by customer(s)
    4: "ABANDONED",// cancelled after assignment but before 'PICKEDUP'
    5: "STARTED",  // status needed by legs
    6: "COMPLETED",
}

var legStatusInt = map[string]int{
	"PLANNED":  0,
	"ASSIGNED": 1,
	"ACCEPTED": 2,
	"REJECTED": 3,
    "ABANDONED":4,
    "STARTED":  5,
    "COMPLETED":6,
}

// -------------- ORDER ------------
type Order struct {
	Id int
	From int `json:"fromStand"`
	To int `json:"toStand"`
    Eta int // set when assigned
    InPool bool
    Cab Cab
    Status string
    MaxWait int // max wait for assignment
    MaxLoss int // [%] loss in Pool
    // LocalDateTime atTime;
    Distance int
}
