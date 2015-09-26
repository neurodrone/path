package path

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	pathURLStub = "http://www.panynj.gov/path/schedules/%s.html"
)

var (
	// The input and output should both be in this format.
	// Two possibilities of this would look like:
	//  1. 5:15AM
	//  2. 12:05PM
	regex = regexp.MustCompile(`\d{1,2}:\d{2}[AP]M`)
)

func (p *Path) pullSchedule(direction string) ([]string, map[string][]string, error) {
	if p.cached && p.stations[direction] != nil && p.stationMap[direction] != nil {
		return p.stations[direction], p.stationMap[direction], nil
	}

	p.stationMap[direction] = make(map[string][]string)

	surl := fmt.Sprintf(pathURLStub, direction)
	url, err := url.Parse(surl)
	if err != nil {
		return nil, nil, err
	}

	client := &http.Client{
		// TODO(@neurodrone): Input this timeout value from command line.
		// by passing it in via the path.New() construct.
		Timeout:   30 * time.Second,
		Transport: http.DefaultTransport,
	}

	resp, err := client.Get(url.String())
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		// Discard the unread response body on error in order to
		// reuse the connection. We always talk to the same frontend
		// so it's a useful practice to bank on the same connection
		// to perform multiple requests, rather than creating a new one
		// everytime.
		//
		// Also, we are not interested if there is an error discarding
		// the remaining part of the response body. Can always recreate
		// the connection in a worst case scenario.
		_, _ = io.Copy(ioutil.Discard, resp.Body)

		return nil, nil, err
	}

	tableNode := p.getTableElem(doc)
	if tableNode == nil {
		return nil, nil, errors.New("no table element found on page")
	}

	stnMap := make(map[int]string)
	for i, column := range p.getColumnNames(tableNode) {
		stnMap[i] = column
	}

	p.stations[direction] = make([]string, 0, len(stnMap))
	schedMap := make(map[int][]string, len(stnMap))

	for i := 0; i < len(stnMap); i++ {
		schedMap[i] = []string{}
		p.stations[direction] = append(p.stations[direction], stnMap[i])
	}

	p.assignStationSchedule(tableNode, schedMap, len(stnMap))

	for i := 0; i < len(stnMap); i++ {
		p.stationMap[direction][stnMap[i]] = schedMap[i]
	}

	p.cached = true

	return p.stations[direction], p.stationMap[direction], nil
}

func (p *Path) getNextTimes(times []string, curTime time.Time, n int) ([]string, error) {
	out := make([]string, 0, n)

	for i, t := range times {
		tm, err := time.Parse(timeFormat, t)
		if err != nil {
			return nil, err
		}

		if !tm.Before(curTime) {
			for n > 0 {
				t = times[i]
				out = append(out, t)

				i++
				if i >= len(times) {
					i = 0
				}
				n--
			}
			return out, nil
		}
	}
	return nil, errors.New("time not found")
}

func (p *Path) assignStationSchedule(tableNode *html.Node, schedMap map[int][]string, n int) {
	index := 0
	// Within <table>
	for c := tableNode.FirstChild; c != nil; c = c.NextSibling {
		// Looking for <tbody>
		if c.Type == html.ElementNode && c.Data == "tbody" {
			// Looping through <tr>
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				// Going through <td>
				for ccc := cc.FirstChild; ccc != nil; ccc = ccc.NextSibling {
					if ccc.FirstChild == nil {
						continue
					}
					data := ccc.FirstChild.Data

					// PATH denotes PM timings within a <strong></strong>
					// tag This accounts for that case.
					if data == "strong" {
						data = ccc.FirstChild.FirstChild.Data
					}

					// There are blocks when no train will arrive at a given
					// station. Skip adding that as a timing value to that
					// station by ensuring that only the values that match
					// the specific pattern are considered to be a valid
					// time value.
					if data == "---" || !regex.MatchString(data) {
						index++
						continue
					}

					// Loop back once we have considered all stations in one go.
					if index >= n {
						index = 0
					}

					schedMap[index] = append(schedMap[index], data)
					index++
				}
			}
		}
	}
}

func (p *Path) getTableElem(root *html.Node) *html.Node {
	if p.isTableNode(root) {
		return root
	}

	for c := root.FirstChild; c != nil; c = c.NextSibling {
		n := p.getTableElem(c)
		if p.isTableNode(n) {
			return n
		}
	}

	return nil
}

func (p *Path) getColumnNames(node *html.Node) []string {
	columnNames := []string{}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "thead" {
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				for ccc := cc.FirstChild; ccc != nil; ccc = ccc.NextSibling {
					if ccc.FirstChild == nil || ccc.FirstChild.FirstChild == nil {
						continue
					}

					columnNames = append(columnNames,
						strings.TrimSpace(ccc.FirstChild.FirstChild.Data))
				}
			}
		}
	}

	return columnNames
}

func (p *Path) isTableNode(node *html.Node) bool {
	return node != nil && node.Type == html.ElementNode && node.Data == "table"
}
