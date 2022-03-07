package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

type jsonNode map[string]interface{}

const title = "Tiger 2"

func main() {
	filename := "/Users/johnjacob/Downloads/Tiger--Latest.json"
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
	w := jsonNode(widget.(map[string]interface{}))
	d := jsonNode(w["definition"].(map[string]interface{}))
	typ := d["type"].(string)
	if typ == "group" {
		widgets := d["widgets"].([]interface{})
		d["widgets"] = transformWidgets(widgets)
		w["definition"] = d
		return w, false
	}
	requests, skip := needsInvocationSection(d)
	if skip {
		return nil, skip
	}

	for i, request := range requests {
		requests[i] = addAlias(request)
	}
	//funcname refers to a filter in datadog which takes the name of lambda
	functionnames := getFuncNames(requests[0])
	errorsSection := createInvocationSection(functionnames)
	requests = append(requests, errorsSection)

	d["requests"] = requests
	w["definition"] = d
	return w, false
}

func slurpJson(filename string) jsonNode {
	dat, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	var j jsonNode
	err = json.Unmarshal(dat, &j)
	if err != nil {
		panic(err)
	}
	return j
}

//needsErrorsSection tells you if you if a given section needs an error section added
func needsErrorsSection(definition jsonNode) ([]jsonNode, bool) {
	typ, ok := (definition)["type"].(string)
	if !ok {
		return nil, true
	}
	if typ != "timeseries" {
		return nil, true
	}
	reqs := (definition["requests"]).([]interface{})
	newReqs := make([]jsonNode, len(reqs))
	for i, req := range reqs {
		newReqs[i] = req.(map[string]interface{})
	}
	if len(reqs) != 1 {
		return nil, true
	}
	return newReqs, false
}

//needsInvocationSection tells you if a given section needs an invocation section
func needsInvocationSection(definition jsonNode) ([]jsonNode, bool) {
	typ, ok := (definition)["type"].(string)
	if !ok {
		return nil, true
	}
	if typ != "timeseries" {
		return nil, true
	}
	reqs := (definition["requests"]).([]interface{})
	newReqs := make([]jsonNode, len(reqs))
	for i, req := range reqs {
		newReqs[i] = req.(map[string]interface{})
	}
	if len(reqs) != 2 {
		return nil, true
	}
	return newReqs, false
}

func getFuncNames(requestSection jsonNode) []string {
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

func concatFuncNames(funcnames []string) string {
	fnamestr := ""
	for _, funcname := range funcnames {
		fnamestr += " functionname:" + funcname
	}
	return fnamestr
}

func createSectionFromTemplate(funcnames []string, template string) jsonNode {
	fnamestr := concatFuncNames(funcnames)
	j := []byte(fmt.Sprintf(template, fnamestr))
	var section jsonNode
	err := json.Unmarshal(j, &section)
	if err != nil {
		panic(err)
	}
	return section
}

var invocationsSectionTemplate = `
          {
            "on_right_yaxis": true,
            "formulas": [
              {
                "alias": "invocations",
                "formula": "query0"
              }
            ],
            "queries": [
              {
                "data_source": "metrics",
                "name": "query0",
                "query": "avg:aws.lambda.invocations{$account,%s}.as_count()"
              }
            ],
            "response_format": "timeseries",
            "style": {
              "palette": "dog_classic",
              "line_type": "solid",
              "line_width": "normal"
            },
            "display_type": "bars"
          }

`

func createInvocationSection(funcnames []string) jsonNode {
	return createSectionFromTemplate(funcnames, invocationsSectionTemplate)
}

var errorsSectionTemplate = `
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
`

func createErrorsSection(funcnames []string) jsonNode {
	return createSectionFromTemplate(funcnames, errorsSectionTemplate)
}
