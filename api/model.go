package main

type Cab struct {
	Id       int
	Location int
	Status   CabStatus
	Name     string
}

type CabStatus int

const (
	ASSIGNED CabStatus = 0
    FREE = 1
    CHARGING = 2
)

func (s CabStatus) String() string {
	switch s {
	case ASSIGNED: 	return "ASSIGNED"
	case FREE:		return "FREE"
	case CHARGING:	return "CHARGING"
	}
	return "unknown"
}

type Stop struct {
	Id 		int
	No 		string
    Name 	string
    Type 	string
    Bearing string
    Latitude 	float64
    Longitude 	float64
}

type Route struct {
	Id int
    Status string
    Legs []Task
}

type Task struct {
	Id int
	FromStand int
	ToStand int 
	Place int
    Status string
}

type Demand struct {
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
