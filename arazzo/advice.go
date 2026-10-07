// Copyright 2023-2026 Princess Beef Heavy Industries, LLC / Dave Shanley
// SPDX-License-Identifier: MIT

package arazzo

import (
	"regexp"
	"strconv"
)

var recommendedName = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

func checkAdvice(v *validation) {
	if !v.opts.advisory {
		return
	}
	emit := func(value, path string) {
		if !recommendedName.MatchString(value) && v.check() {
			v.emit(Diagnostic{Code: CodeAdvisory, Severity: SeverityWarning, Message: "name does not follow the recommended programming convention", Location: v.location(path)})
		}
	}
	for i, source := range array(v.root["sourceDescriptions"]) {
		emit(text(object(source)["name"]), "/sourceDescriptions/"+strconv.Itoa(i)+"/name")
	}
	for i, workflow := range array(v.root["workflows"]) {
		path := "/workflows/" + strconv.Itoa(i)
		emit(text(object(workflow)["workflowId"]), path+"/workflowId")
		for j, step := range array(object(workflow)["steps"]) {
			emit(text(object(step)["stepId"]), path+"/steps/"+strconv.Itoa(j)+"/stepId")
		}
	}
}
