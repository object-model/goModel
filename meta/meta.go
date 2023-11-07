package meta

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"strings"
)

var validType = map[string]struct{}{
	"bool":   struct{}{},
	"int":    struct{}{},
	"uint":   struct{}{},
	"float":  struct{}{},
	"string": struct{}{},
	"array":  struct{}{},
	"slice":  struct{}{},
	"struct": struct{}{},
	"meta":   struct{}{},
}

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
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Element     *ParamMeta  `json:"element"`
	Fields      []ParamMeta `json:"fields"`
	Length      uint        `json:"length"`
	Unit        string      `json:"unit"`
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
	Name        string       `json:"name"` // 物模型名称
	Description string       `json:"description"`
	State       []ParamMeta  `json:"state"`
	Event       []EventMeta  `json:"event"`
	Method      []MethodMeta `json:"method"`
}

func (m *Meta) AllStates() []string {
	res := make([]string, 0, len(m.State))
	for i := range m.State {
		res = append(res, strings.Join([]string{
			m.Name,
			m.State[i].Name,
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

func (m *Meta) Check() error {

	if m.Name == "" {
		return fmt.Errorf("root: name NOT exist")
	}

	if m.Description == "" {
		return fmt.Errorf("root: description NOT exist")
	}

	if m.State == nil {
		return fmt.Errorf("root: has NO state")
	}

	// 检查每个状态
	visited := make(map[string]struct{})
	for i, stateMeta := range m.State {
		err := checkState(stateMeta, visited)
		if err != nil {
			return fmt.Errorf("state[%d]: %s", i, err)
		}
	}

	// 检查每个事件
	visited = make(map[string]struct{})
	for i, eventMeta := range m.Event {
		if err := checkEvent(eventMeta, visited); err != nil {
			return fmt.Errorf("event[%d]: %s", i, err)
		}
	}

	// 检查每个方法
	visited = make(map[string]struct{})
	for i, methodMeta := range m.Method {
		if err := checkMethod(methodMeta, visited); err != nil {
			return fmt.Errorf("method[%d]: %s", i, err)
		}
	}

	return nil
}

func checkState(state ParamMeta, visited map[string]struct{}) error {
	if err := checkParamInfo(state, false); err != nil {
		return err
	}

	// 确保状态名不重复
	name := state.Name
	if _, seen := visited[name]; seen {
		return fmt.Errorf("repeat state name: %q", name)
	} else {
		visited[name] = struct{}{}
	}

	return nil
}

func checkEvent(event EventMeta, visited map[string]struct{}) error {
	if event.Name == "" {
		return fmt.Errorf("name NOT exist")
	}

	if event.Description == "" {
		return fmt.Errorf("description NOT exist")
	}

	// 确保参数必须存在
	if event.Args == nil {
		return fmt.Errorf("no args")
	}

	argsName := make(map[string]struct{})
	for i, arg := range event.Args {
		// 检查事件参数本身
		if err := checkParamInfo(arg, false); err != nil {
			return fmt.Errorf("args[%d]: %s", i, err)
		}

		// 确保参数名称不重复
		name := arg.Name
		if _, seen := argsName[name]; seen {
			return fmt.Errorf("args[%d]: repeat arg name: %q", i, name)
		} else {
			argsName[name] = struct{}{}
		}
	}

	// 确保事件名不重复
	eventName := event.Name
	if _, seen := visited[eventName]; seen {
		return fmt.Errorf("repeat event name: %q", eventName)
	} else {
		visited[eventName] = struct{}{}
	}

	return nil
}

func checkMethod(method MethodMeta, visited map[string]struct{}) error {
	if method.Name == "" {
		return fmt.Errorf("name NOT exist")
	}

	if method.Description == "" {
		return fmt.Errorf("description NOT exist")
	}

	// 方法元信息必须有args字段
	if method.Args == nil {
		return fmt.Errorf("no args")
	}

	// 逐个检查每个参数
	argsName := make(map[string]struct{})
	for i, arg := range method.Args {
		// 检查事件参数本身
		if err := checkParamInfo(arg, false); err != nil {
			return fmt.Errorf("args[%d]: %s", i, err)
		}

		// 确保参数名称不重复
		name := arg.Name
		if _, seen := argsName[name]; seen {
			return fmt.Errorf("args[%d]: repeat arg name: %q", i, name)
		} else {
			argsName[name] = struct{}{}
		}
	}

	// 方法元信息必须有response字段
	if method.Response == nil {
		return fmt.Errorf("no response")
	}

	// 逐个检查每个返回值
	respName := make(map[string]struct{})
	for i, resp := range method.Response {
		// 检查事件参数本身
		if err := checkParamInfo(resp, false); err != nil {
			return fmt.Errorf("response[%d]: %s", i, err)
		}

		// 确保参数名称不重复
		name := resp.Name
		if _, seen := respName[name]; seen {
			return fmt.Errorf("response[%d]: repeat resp name: %q", i, name)
		} else {
			respName[name] = struct{}{}
		}
	}

	// 确保事件名不重复
	methodName := method.Name
	if _, seen := visited[methodName]; seen {
		return fmt.Errorf("repeat method name: %q", methodName)
	} else {
		visited[methodName] = struct{}{}
	}

	return nil
}

func checkParamInfo(paramMeta ParamMeta, isElement bool) error {
	// 不是element情况下检查name和description字段
	if !isElement {
		if paramMeta.Name == "" {
			return fmt.Errorf("name NOT exist")
		}

		if paramMeta.Description == "" {
			return fmt.Errorf("description NOT exist")
		}
	}

	if paramMeta.Type == "" {
		return fmt.Errorf("type NOT exist")
	}

	// type字段的值必须有效
	if !isValidType(paramMeta.Type) {
		return fmt.Errorf("invalid type: %q", paramMeta.Type)
	}

	switch paramMeta.Type {
	case "array":
		// length必须大于0
		if paramMeta.Length == 0 {
			return fmt.Errorf("length is NOT greater than 0")
		}
		// 数组类型必须有element字段
		if paramMeta.Element == nil {
			return fmt.Errorf("element NOT exist")
		}

		if err := checkParamInfo(*paramMeta.Element, true); err != nil {
			return fmt.Errorf("element: %s", err)
		}
	case "struct":
		// 结构体类型必须有fields字段
		if paramMeta.Fields == nil {
			return fmt.Errorf("fields NOT exist")
		}
		filedNameSet := make(map[string]interface{})
		for i, fieldMeta := range paramMeta.Fields {
			// 检查字段本身
			if err := checkParamInfo(fieldMeta, false); err != nil {
				return fmt.Errorf("fields[%d]: %s", i, err)
			}

			// 确保字段名不重复
			fieldName := fieldMeta.Name
			if _, seen := filedNameSet[fieldName]; seen {
				return fmt.Errorf("fields[%d]: repeat field name: %q", i, fieldName)
			} else {
				filedNameSet[fieldName] = struct{}{}
			}
		}
	case "slice":
		// 数组类型必须有elem字段
		if paramMeta.Element == nil {
			return fmt.Errorf("element NOT exist")
		}

		if err := checkParamInfo(*paramMeta.Element, true); err != nil {
			return fmt.Errorf("element: %s", err)
		}
	}

	// 如果存在range字段，则对range字段值进行检查
	if paramMeta.Range != nil {
		return checkRange(*paramMeta.Range, paramMeta.Type)
	}

	return nil
}

func isValidType(Type string) bool {
	_, seen := validType[Type]
	return seen
}

func checkRange(rangeMeta RangeInfo, Type string) error {
	switch Type {
	case "string":
		return checkStringRange(rangeMeta)
	case "float":
		return checkFloatRange(rangeMeta)
	case "int":
		return checkIntRange(rangeMeta)
	case "uint":
		return checkUintRange(rangeMeta)
	default:
		break
	}

	return fmt.Errorf("range: %q NOT support range", Type)
}

func checkStringRange(rangeMeta RangeInfo) error {
	if rangeMeta.Option == nil {
		return fmt.Errorf("no option for string range")
	}
	valueSet := make(map[string]struct{})
	for i, option := range rangeMeta.Option {
		// 每个option选项必须包含description
		if option.Description == "" {
			return fmt.Errorf("range: option[%d]: NO description", i)
		}

		// 每个option选项必须包含value
		if option.Value == nil {
			return fmt.Errorf("range: option[%d]: NO value", i)
		}

		// 每个option选项包含的value必须是string类型
		var value string
		if jsoniter.Unmarshal(option.Value, &value) != nil {
			return fmt.Errorf("range: option[%d]: value is NOT string", i)
		}

		// 每个option选项的value值不能重复
		if _, seen := valueSet[value]; seen {
			return fmt.Errorf("range: option[%d]: repeat valeu: %q", i, value)
		} else {
			valueSet[value] = struct{}{}
		}
	}

	// 如果有default字段，检查默认值是否合理
	if rangeMeta.Default != nil {
		// 默认值必须是字符串
		var defaultValue string
		if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
			return fmt.Errorf("range: default: NOT string")
		}

		// 默认值必须在可选值列表中
		if _, seen := valueSet[defaultValue]; !seen {
			return fmt.Errorf("range: default: %q NOT in option", defaultValue)
		}
	}

	return nil
}

func checkFloatRange(rangeMeta RangeInfo) error {
	// float类型的range必须有min 或 max字段, 不能两个都没有
	if rangeMeta.Min == nil && rangeMeta.Max == nil {
		return fmt.Errorf("range: NO min or max for float range")
	}

	var max float64
	var min float64

	// 在有min字段情况下, float类型的min字段必须是double类型
	if rangeMeta.Min != nil {
		if jsoniter.Unmarshal(rangeMeta.Min, &min) != nil {
			return fmt.Errorf("range: min is NOT float")
		}
	}

	// 在有max字段情况下, float类型的max字段必须是double类型
	if rangeMeta.Max != nil {
		if jsoniter.Unmarshal(rangeMeta.Max, &max) != nil {
			return fmt.Errorf("range: max is NOT float")
		}
	}

	// 在max和min字段都存在的情况下，最小值一定严格小于最大值
	if rangeMeta.Min != nil && rangeMeta.Max != nil {
		if min > max {
			return fmt.Errorf("range: min is NOT less than max")
		}
	}

	// 如果有default字段，检查默认值是否合理
	if rangeMeta.Default != nil {
		var defaultValue float64
		if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
			return fmt.Errorf("range: default: NOT float")
		}

		// 默认值必须介于[min, max]之间
		if rangeMeta.Min != nil && defaultValue < min {
			return fmt.Errorf("range: default: less than min")
		}

		if rangeMeta.Max != nil && defaultValue > max {
			return fmt.Errorf("range: default: greater than max")
		}
	}

	return nil
}

func checkIntRange(rangeMeta RangeInfo) error {
	// 如果int类型的range有option字段，则以option为准
	// 否则以最大值max、最小值min为准
	if rangeMeta.Option != nil {
		valueSet := make(map[int]struct{})
		for i, option := range rangeMeta.Option {
			// 每个option选项必须包含description
			if option.Description == "" {
				return fmt.Errorf("range: option[%d]: NO description", i)
			}
			// 每个option选项必须包含value
			if option.Value == nil {
				return fmt.Errorf("range: option[%d]: NO value", i)
			}

			// 每个option选项包含的value必须是int类型
			var value int
			if jsoniter.Unmarshal(option.Value, &value) != nil {
				return fmt.Errorf("range: option[%d]: value is NOT int", i)
			}

			// 每个option选项的value值不能重复
			if _, seen := valueSet[value]; seen {
				return fmt.Errorf("range: option[%d]: repeat valeu: %q", i, value)
			} else {
				valueSet[value] = struct{}{}
			}
		}

		// 如果有default字段，检查默认值是否合理
		if rangeMeta.Default != nil {
			// 默认值必须是字符串
			var defaultValue int
			if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
				return fmt.Errorf("range: default: NOT int")
			}

			// 默认值必须在可选值列表中
			if _, seen := valueSet[defaultValue]; !seen {
				return fmt.Errorf("range: default: %q NOT in option", defaultValue)
			}
		}
	} else {
		// int类型的range必须有min 或 max字段, 不能两个都没有
		if rangeMeta.Min == nil && rangeMeta.Max == nil {
			return fmt.Errorf("range: NO min or max for int range")
		}

		var max int
		var min int

		// 在有min字段情况下, int类型的min字段必须是int类型
		if rangeMeta.Min != nil {
			if jsoniter.Unmarshal(rangeMeta.Min, &min) != nil {
				return fmt.Errorf("range: min is NOT int")
			}
		}

		// 在有max字段情况下, int类型的max字段必须是int类型
		if rangeMeta.Max != nil {
			if jsoniter.Unmarshal(rangeMeta.Min, &max) != nil {
				return fmt.Errorf("range: max is NOT int")
			}
		}

		// 在max和min字段都存在的情况下，最小值一定严格小于最大值
		if rangeMeta.Min != nil && rangeMeta.Max != nil {
			if min > max {
				return fmt.Errorf("range: min is NOT less than max")
			}
		}

		// 如果有default字段，检查默认值是否合理
		if rangeMeta.Default != nil {
			var defaultValue int
			if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
				return fmt.Errorf("range: default: NOT int")
			}

			// 默认值必须介于[min, max]之间
			if rangeMeta.Min != nil && defaultValue < min {
				return fmt.Errorf("range: default: less than min")
			}

			if rangeMeta.Max != nil && defaultValue > max {
				return fmt.Errorf("range: default: greater than max")
			}
		}
	}
	return nil
}

func checkUintRange(rangeMeta RangeInfo) error {
	// 如果uint类型的range有option字段，则以option为准
	// 否则以最大值max、最小值min为准
	if rangeMeta.Option != nil {
		valueSet := make(map[uint]struct{})
		for i, option := range rangeMeta.Option {
			// 每个option选项必须包含value
			if option.Value == nil {
				return fmt.Errorf("range: option[%d]: NO value", i)
			}

			// 每个option选项包含的value必须是uint类型
			var value uint
			if jsoniter.Unmarshal(option.Value, &value) != nil {
				return fmt.Errorf("range: option[%d]: value is NOT uint", i)
			}

			// 每个option选项的value值不能重复
			if _, seen := valueSet[value]; seen {
				return fmt.Errorf("range: option[%d]: repeat valeu: %q", i, value)
			} else {
				valueSet[value] = struct{}{}
			}
		}

		// 如果有default字段，检查默认值是否合理
		if rangeMeta.Default != nil {
			// 默认值必须是字符串
			var defaultValue uint
			if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
				return fmt.Errorf("range: default: NOT uint")
			}

			// 默认值必须在可选值列表中
			if _, seen := valueSet[defaultValue]; !seen {
				return fmt.Errorf("range: default: %q NOT in option", defaultValue)
			}
		}
	} else {
		// uint类型的range必须有min 或 max字段, 不能两个都没有
		if rangeMeta.Min == nil && rangeMeta.Max == nil {
			return fmt.Errorf("range: NO min or max for int range")
		}

		var max uint
		var min uint

		// 在有min字段情况下, uint类型的min字段必须是uint类型
		if rangeMeta.Min != nil {
			if jsoniter.Unmarshal(rangeMeta.Min, &min) != nil {
				return fmt.Errorf("range: min is NOT uint")
			}
		}

		// 在有max字段情况下, uint类型的max字段必须是uint类型
		if rangeMeta.Max != nil {
			if jsoniter.Unmarshal(rangeMeta.Min, &max) != nil {
				return fmt.Errorf("range: max is NOT uint")
			}
		}

		// 在max和min字段都存在的情况下，最小值一定严格小于最大值
		if rangeMeta.Min != nil && rangeMeta.Max != nil {
			if min > max {
				return fmt.Errorf("range: min is NOT less than max")
			}
		}

		// 如果有default字段，检查默认值是否合理
		if rangeMeta.Default != nil {
			var defaultValue uint
			if jsoniter.Unmarshal(rangeMeta.Default, &defaultValue) != nil {
				return fmt.Errorf("range: default: NOT uint")
			}

			// 默认值必须介于[min, max]之间
			if rangeMeta.Min != nil && defaultValue < min {
				return fmt.Errorf("range: default: less than min")
			}

			if rangeMeta.Max != nil && defaultValue > max {
				return fmt.Errorf("range: default: greater than max")
			}
		}
	}
	return nil
}
