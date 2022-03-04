package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

type jsonRoot map[string]interface{}

const title = "Tiger 2"

func main() {
	filename := "/Users/johnjacob/Downloads/Tiger--2022-03-04T19_45_07.json"
	j := slurpJson(filename)
	j["id"] = "" //I'm not sure what id does, so make it blank
	j["title"] = title
	widgets := (j)["widgets"].([]interface{})
	j["widgets"] = transformWidgets(widgets)
	s, _ := json.Marshal(j)
	outpath := "/Users/johnjacob/Desktop/datadog_new_dashboard.json"
	err := os.WriteFile(outpath, s, 0666)
	if err != nil {
		panic(err)
	}
}

func transformWidgets(widgets []interface{}) []interface{} {
	for i, widget := range widgets {
		w, skip := transformWidget(widget)
		if skip {
			continue
		}
		widgets[i] = w
	}
	return widgets
}

func addAlias(request map[string]interface{}) map[string]interface{} {
	formulas := request["formulas"].([]interface{})
	formula := formulas[0].(map[string]interface{})
	formula["alias"] = "duration"
	formulas[0] = formula
	request["formulas"] = formulas
	return request
}

func transformWidget(widget interface{}) (interface{}, bool) {
	//if the requests array has length 1
	w := jsonRoot(widget.(map[string]interface{}))
	d := jsonRoot(w["definition"].(map[string]interface{}))
	typ := d["type"].(string)
	if typ == "group" {
		widgets := d["widgets"].([]interface{})
		d["widgets"] = transformWidgets(widgets)
		w["definition"] = d
		return w, false
	}
	requests, skip := isValidRequestsSection(d)
	if skip {
		return nil, skip
	}

	for i, request := range requests {
		requests[i] = addAlias(request)
	}
	//funcname refers to a filter in datadog which takes the name of lambda
	functionnames := getFuncNames(requests[0])
	errorsSection := getErrorsSection(functionnames)
	requests = append(requests, errorsSection)

	d["requests"] = requests
	w["definition"] = d
	return w, false
}

func slurpJson(filename string) jsonRoot {
	dat, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	var j jsonRoot
	err = json.Unmarshal(dat, &j)
	if err != nil {
		panic(err)
	}
	return j
}

//isValidRequestsSection tells you if you want to change the widget
func isValidRequestsSection(definition jsonRoot) ([]jsonRoot, bool) {
	typ, ok := (definition)["type"].(string)
	if !ok {
		return nil, true
	}
	if typ != "timeseries" {
		return nil, true
	}
	reqs := (definition["requests"]).([]interface{})
	newReqs := make([]jsonRoot, len(reqs))
	for i, req := range reqs {
		newReqs[i] = jsonRoot(req.(map[string]interface{}))
	}
	if len(reqs) != 1 {
		return nil, true
	}
	return newReqs, false
}

func getFuncNames(requestSection jsonRoot) []string {
	r := regexp.MustCompile(`.*functionname:([^,}]*)`)
	queriesSection, ok := (requestSection["queries"]).([]interface{})
	funcnames := make([]string, 0)
	if !ok {
		return funcnames
	}
	for _, query := range queriesSection {
		queryM := query.(map[string]interface{})
		queryQuery := queryM["query"].(string)
		matches := r.FindAllStringSubmatch(queryQuery, -1)
		for _, match := range matches {
			funcname := match[1]
			funcnames = append(funcnames, funcname)
		}
	}
	return funcnames
}

func getErrorsSection(funcnames []string) jsonRoot {
	fnamestr := ""
	for _, funcname := range funcnames {
		fnamestr += " functionname:" + funcname
	}
	template := []byte(fmt.Sprintf(`
	          {
            "on_right_yaxis": true,
            "formulas": [
              {
				"alias": "error logs",
                "formula": "query0"
              }
            ],
            "queries": [
              {
                "data_source": "logs",
                "name": "query0",
                "search": {
                  "query": "$account status:error %s"
                },
                "indexes": [
                  "*"
                ],
                "compute": {
                  "aggregation": "count"
                },
                "group_by": []
              }
            ],
            "response_format": "timeseries",
            "style": {
              "palette": "red",
              "line_type": "solid",
              "line_width": "normal"
            },
            "display_type": "bars"
          }
`, fnamestr))
	var errorsSection jsonRoot
	err := json.Unmarshal(template, &errorsSection)
	if err != nil {
		panic(err)
	}
	return errorsSection
}
