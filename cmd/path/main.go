package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/neurodrone/path"
)

func main() {
	var (
		port  = flag.String("port", "", "http port to start the service on")
		limit = flag.Int("limit", 5, "limit on no. of results returned for times")
	)
	flag.Parse()

	p, err := path.New(*limit)
	if err != nil {
		log.Fatalf("cannot create path object: %s", err)
	}

	r := mux.NewRouter()

	// The format we expect an input in is:
	//  1. Station you are presently at. (the ones you want the time for)
	//  2. The direction you are traveling in.
	//  3. Your wallclock time, so (*limit) no. of results can be returned
	//     to you. And also because your wallclock != server's wallclock.
	//     If we always returned next (*limit) results from the server's
	//     point-of-view you could very well end up missing the next train,
	//     which you could have easily caught. If you don't think this would
	//     happen or are plainly confused, there's some reading material for
	//     you here: https://queue.acm.org/detail.cfm?id=2745385.
	r.HandleFunc("/p/{stn}/{direction}/{time}/", p.GrabTimes).Methods("GET")

	// If you just want the list of stations for the direction you intend to
	// travel in, you can use this endpoint.
	r.HandleFunc("/p/list/{direction}/", p.ListStations).Methods("GET")

	log.Printf("Starting Path Server on :%s", *port)
	if err := http.ListenAndServe(":"+*port, r); err != nil {
		log.Fatalf("cannot start http server: %s", err)
	}
}
