package path

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const (
	// MaxTimesLimit maintains the upper-limit to how many values
	// can be returned for scheduled train timings.
	MaxTimesLimit = 20

	// timeFormat gives the format of input and output time variables.
	timeFormat = "3:04PM"

	// HTTPErrPrefix is used to prefix all errors so that they are
	// easily recognizable when read by an embedded device.
	HTTPErrPrefix = "error: "
)

var (
	// destMap translates the given direction into the page path
	// at which the schedule for a train route is available.
	destMap = map[string]string{
		"jsq_33rd": "JSQ_33rd_Weekday",
		"33rd_jsq": "33rd_JSQ_Weekday",
	}

	// ErrInvalidLimit is called when no. of timings that are demanded
	// are greater than the limit stated within MaxTimesLimit.
	ErrInvalidLimit = errors.New("invalid limit for times provided")
)

// Path provides the necessary functionality to parse the PATH train schedule
// from the Port Authority of New York and New Jersey website and generates
// results in a way that are easily understandable by embedded devices like
// Pebble Smartwatch, etc.
type Path struct {
	timesLimit int

	// Map of [direction of travel] of [stations] of array times.
	stationMap map[string]map[string][]string

	// Map of [direction of travel] of [list of stations in order]
	stations map[string][]string

	// True if results are previously cached and can be retrieved locally.
	cached bool
}

// New creates a new instance of Path. Once Path is instantiated correctly
// it can be used to query schedule from the PATH website.
func New(limit int) (*Path, error) {
	if limit < 0 || limit > MaxTimesLimit {
		return nil, ErrInvalidLimit
	}

	return &Path{
		timesLimit: limit,
		stationMap: make(map[string]map[string][]string),
		stations:   make(map[string][]string),
	}, nil
}

// ListStations is a HTTP handler that prints out the list of station names
// that lie between first and last station, and in that order.
func (p *Path) ListStations(w http.ResponseWriter, r *http.Request) {
	direction := mux.Vars(r)["direction"]
	if direction == "" {
		http.Error(w, HTTPErrPrefix+"'direction' cannot be empty", http.StatusBadRequest)
		return
	}

	endURLStub, ok := destMap[direction]
	if !ok {
		http.Error(w, HTTPErrPrefix+fmt.Sprintf("unable to find loc: %q", direction), http.StatusInternalServerError)
		return
	}

	stations, _, err := p.pullSchedule(endURLStub)
	if err != nil {
		http.Error(w, HTTPErrPrefix+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte{'['})
	w.Write([]byte(strings.Join(stations, ",")))
	w.Write([]byte{']'})
}

// GrabTimes is a HTTP handler that will grab a list of N timings when
// the next trains to your desired destination will arrive.
func (p *Path) GrabTimes(w http.ResponseWriter, r *http.Request) {
	stn := mux.Vars(r)["stn"]
	if stn == "" {
		http.Error(w, HTTPErrPrefix+"'stn' cannot be empty", http.StatusBadRequest)
		return
	}

	direction := mux.Vars(r)["direction"]
	if direction == "" {
		http.Error(w, HTTPErrPrefix+"'direction' cannot be empty", http.StatusBadRequest)
		return
	}

	tyme := mux.Vars(r)["time"]
	if tyme == "" {
		http.Error(w, HTTPErrPrefix+"'time' cannot be empty", http.StatusBadRequest)
		return
	}

	endURLStub, ok := destMap[direction]
	if !ok {
		http.Error(w, HTTPErrPrefix+fmt.Sprintf("unable to find loc: %q", direction), http.StatusInternalServerError)
		return
	}

	_, stationMap, err := p.pullSchedule(endURLStub)
	if err != nil {
		http.Error(w, HTTPErrPrefix+err.Error(), http.StatusInternalServerError)
		return
	}

	times, ok := stationMap[stn]
	if !ok {
		http.Error(w, HTTPErrPrefix+fmt.Sprintf("invalid stn: %q", stn), http.StatusBadRequest)
		return
	}

	curTime, err := time.Parse(timeFormat, tyme)
	if err != nil {
		http.Error(w, HTTPErrPrefix+fmt.Sprintf("invalid 'time': %s", err), http.StatusBadRequest)
		return
	}

	times, err = p.getNextTimes(times, curTime, p.timesLimit)
	if err != nil {
		http.Error(w, HTTPErrPrefix+err.Error(), http.StatusInternalServerError)
		return
	}

	dur := make([]string, 0, len(times))
	for _, t := range times {
		tt, _ := time.Parse(timeFormat, t)

		timeDiff := tt.Sub(curTime)
		if tt.Before(curTime) {
			timeDiff = tt.Add(24 * time.Hour).Sub(curTime)
		}
		dur = append(dur, strconv.Itoa(int(timeDiff.Minutes())))
	}

	var buf bytes.Buffer
	for i := 0; i < len(times); i++ {
		buf.WriteString(fmt.Sprintf("%s,%s mins left;", times[i], dur[i]))
	}

	w.Write(buf.Bytes())
}
