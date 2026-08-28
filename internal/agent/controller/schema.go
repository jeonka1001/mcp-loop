package controller

import "github.com/google/jsonschema-go/jsonschema"

// promptProperties는 세 질의 툴이 공유하는 프롬프트 관련 인자다.
func promptProperties() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"prompt": {Type: "string", Description: "에이전트에 전달할 질문 또는 지시문."},
		"model":  {Type: "string", Description: "선택 사항. 해당 CLI가 지원하는 모델명. 미지정 시 CLI 기본 모델."},
		"timeout_ms": {
			Type:        "integer",
			Description: "선택 사항. 실행 타임아웃(ms). 5000~900000.",
		},
		"cwd": {Type: "string", Description: "선택 사항. CLI를 실행할 작업 디렉터리 절대 경로."},
	}
}

// askAgentSchema는 agent를 필수 enum으로 강제한다. 기본값이 없으므로 미지정 호출은 불가능하다.
func askAgentSchema(ids []string) *jsonschema.Schema {
	properties := promptProperties()
	properties["agent"] = &jsonschema.Schema{
		Type:        "string",
		Enum:        toAnySlice(ids),
		Description: "질의를 보낼 에이전트 id. 필수.",
	}
	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"agent", "prompt"},
	}
}

// askAgentsSchema는 agents 배열을 필수로 강제한다(최소 1개). 전체 자동 호출 경로는 없다.
func askAgentsSchema(ids []string) *jsonschema.Schema {
	minItems := 1
	properties := promptProperties()
	properties["agents"] = &jsonschema.Schema{
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string", Enum: toAnySlice(ids)},
		MinItems:    &minItems,
		Description: "질의를 보낼 에이전트 id 목록. 필수.",
	}
	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"agents", "prompt"},
	}
}

func toAnySlice(values []string) []any {
	converted := make([]any, 0, len(values))
	for _, value := range values {
		converted = append(converted, value)
	}
	return converted
}
