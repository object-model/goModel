package meta

import (
	jsoniter "github.com/json-iterator/go"
	"strings"
)

type OptionInfo struct {
	Value       jsoniter.RawMessage `json:"value"`
	Description string              `json:"description"`
}

type RangeInfo struct {
	Max     jsoniter.RawMessage `json:"max"`
	Min     jsoniter.RawMessage `json:"min"`
	Option  []OptionInfo        `json:"option"`
	Default jsoniter.RawMessage `json:"default"`
}

type ParamMeta struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	Type        string      `json:"type"`
	Element     *ParamMeta  `json:"element"`
	Fields      []ParamMeta `json:"fields"`
	Length      *uint       `json:"length"`
	Unit        *string     `json:"unit"`
	Range       *RangeInfo  `json:"range"`
}

type EventMeta struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Args        []ParamMeta `json:"args"`
}

type MethodMeta struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Args        []ParamMeta `json:"args"`
	Response    []ParamMeta `json:"response"`
}

type Meta struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	State       []ParamMeta  `json:"state"`
	Event       []EventMeta  `json:"event"`
	Method      []MethodMeta `json:"method"`
}

func (m *Meta) AllStates() []string {
	res := make([]string, 0, len(m.State))
	for i := range m.State {
		res = append(res, strings.Join([]string{
			m.Name,
			*(m.State[i].Name),
		}, "/"))
	}
	return res
}

func (m *Meta) AllEvents() []string {
	res := make([]string, 0, len(m.Event))
	for i := range m.Event {
		res = append(res, strings.Join([]string{
			m.Name,
			m.Event[i].Name,
		}, "/"))
	}
	return res
}
