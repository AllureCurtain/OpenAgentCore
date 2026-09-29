package modeltransport

import (
	"strings"

	"github.com/tidwall/gjson"
)

// Inspect protocol blocks, never arbitrary text or function arguments. The SDK
// owns conversion; this gate only rejects unqualified capability families.
func requestRequirements(protocol Protocol, raw []byte) Requirements {
	root := gjson.ParseBytes(raw)
	required := Requirements{}
	var content func(gjson.Result)
	content = func(parts gjson.Result) {
		for _, part := range parts.Array() {
			switch part.Get("type").String() {
			case "image", "image_url", "input_image":
				required.Images = true
			case "tool_result":
				content(part.Get("content"))
			}
		}
	}
	if protocol == Responses {
		for _, item := range root.Get("input").Array() {
			content(item.Get("content"))
			if item.Get("type").String() == "function_call_output" || item.Get("type").String() == "custom_tool_call_output" {
				content(item.Get("output"))
			}
		}
	} else {
		for _, message := range root.Get("messages").Array() {
			content(message.Get("content"))
		}
	}
	if protocol == Responses {
		required.StructuredOutput = root.Get("text.format.type").String() == "json_schema"
		v := root.Get("text.verbosity").String()
		required.NonDefaultVerbosity = v != "" && v != "medium"
	} else if protocol == ChatCompletions {
		required.StructuredOutput = root.Get("response_format.type").String() == "json_schema"
	} else {
		required.StructuredOutput = root.Get("output_config.format").Exists() || root.Get("output_format").Exists()
	}
	for _, tool := range root.Get("tools").Array() {
		kind := tool.Get("type").String()
		if strings.HasPrefix(kind, "web_search") {
			required.WebSearch = true
		}
		if strings.HasPrefix(kind, "tool_search") || tool.Get("defer_loading").Bool() {
			required.ToolSearch = true
		}
	}
	return required
}
