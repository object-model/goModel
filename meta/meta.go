package meta

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"strings"
)

var validType = map[string]struct{}{
	"bool":   {},
	"int":    {},
	"uint":   {},
	"float":  {},
	"string": {},
	"array":  {},
	"slice":  {},
	"struct": {},
	"meta":   {},
}

type OptionInfo struct {
	Value       interface{} `json:"value"`
	Description string      `json:"description"`
}

type RangeInfo struct {
	Max     interface{}  `json:"max"`
	Min     interface{}  `json:"min"`
	Option  []OptionInfo `json:"option"`
	Default interface{}  `json:"default"`
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
	Name        string       `json:"name"`        // 物模型名称
	Description string       `json:"description"` // 物模型描述
	State       []ParamMeta  `json:"state"`       // 状态元信息
	Event       []EventMeta  `json:"event"`       // 事件元信息
	Method      []MethodMeta `json:"method"`      // 方法元信息

	nameTokens    []string       // 物模型名称以/分割后的有效token
	nameTemplates map[string]int // 模板参数名到nameTokens中的索引
}

type TemplateParam map[string]string

func (m *Meta) AllStates() []string {
	res := make([]string, 0, len(m.State))
	for i := range m.State {
		res = append(res, strings.Join([]string{
			m.Name,
			*m.State[i].Name,
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

func (m *Meta) parseTemplate(name string) {
	// 1.先以/分割
	tokens := strings.Split(name, "/")

	// 2.去除每个token前后的空格
	for i, token := range tokens {
		tokens[i] = strings.Trim(token, " \t\n\r\f\v")
	}

	// 3.过滤空的token
	m.nameTokens = make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != "" {
			m.nameTokens = append(m.nameTokens, token)
		}
	}

	// 4.查找模板参数
	m.nameTemplates = make(map[string]int)
	for i, token := range m.nameTokens {
		if strings.HasPrefix(token, "{") {
			// 去除{和 } 并 去除空格后的 模板名称
			templateName := strings.TrimSpace(token[1 : len(token)-1])

			// 记录模板参数的下标位置
			m.nameTemplates[templateName] = i

			// 模板值暂时替换成空格
			m.nameTokens[i] = ""
		}
	}
}

func (m *Meta) setTemplate(param TemplateParam) (err error) {
	for name, index := range m.nameTemplates {
		if val, seen := param[name]; !seen {
			// 模板参数不存在则报错
			err = fmt.Errorf("template %q: missing", name)
		} else if val == "" {
			// 设置的模板参数名不能为空
			err = fmt.Errorf("template %q: value is empty", name)
		} else {
			m.nameTokens[index] = val
		}
	}

	// NOTE: 保存失败要手动清空缓存的模板参数
	// NOTE: 否则会导致元信息还是空，却存在模板参数
	if err != nil {
		m.nameTokens = nil
		m.nameTemplates = nil
	}

	return
}

func Parse(rawData []byte, templateParam TemplateParam) (Meta, error) {
	// 1. 读取
	it := jsoniter.ParseBytes(jsoniter.ConfigDefault, rawData)
	root := it.ReadAny()
	if it.Error != nil || it.WhatIsNext() != jsoniter.InvalidValue {
		return Meta{}, fmt.Errorf("parse JSON failed")
	}

	// 2. 检查元信息是否正确
	if err := check(root); err != nil {
		return Meta{}, err
	}

	// 3. 解析
	ans := Meta{
		Description: strings.TrimSpace(root.Get("description").ToString()),
		State:       make([]ParamMeta, 0, root.Get("state").Size()),
		Event:       make([]EventMeta, 0, root.Get("event").Size()),
		Method:      make([]MethodMeta, 0, root.Get("method").Size()),
	}

	// 4.解析模板参数
	ans.parseTemplate(root.Get("name").ToString())

	// 3.规范化模板参数
	templateParam = trimTemplate(templateParam)

	// 5.保存模板参数
	if err := ans.setTemplate(templateParam); err != nil {
		return Meta{}, err
	}

	// 6.更新模型名称
	ans.Name = strings.Join(ans.nameTokens, "/")

	for i := 0; i < root.Get("state").Size(); i++ {
		ans.State = append(ans.State, createParamMeta(root.Get("state").Get(i)))
	}

	for i := 0; i < root.Get("event").Size(); i++ {
		ans.Event = append(ans.Event, createEventMeta(root.Get("event").Get(i)))
	}

	for i := 0; i < root.Get("method").Size(); i++ {
		ans.Method = append(ans.Method, createMethodMeta(root.Get("method").Get(i)))
	}

	return ans, nil
}

func check(root jsoniter.Any) error {
	// 根节点必须是对象类型
	if root.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("root: NOT an object")
	}

	// 检查name和description字段
	if err := checkNameDesc(root); err != nil {
		return fmt.Errorf("root: %s", err)
	}

	// 检查模型名称是否符合规范
	if err := checkModelName(root.Get("name").ToString()); err != nil {
		return fmt.Errorf("root: name: %s", err)
	}

	// 必须包含state字段
	state := root.Get("state")
	if state.LastError() != nil {
		return fmt.Errorf("root: state NOT exist")
	}

	// state必须是数组类型
	if state.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("root: state is NOT array")
	}

	// 必须包含event字段
	event := root.Get("event")
	if event.LastError() != nil {
		return fmt.Errorf("root: event NOT exist")
	}

	// event必须是数组类型
	if event.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("root: event is NOT array")
	}

	// 必须包含method字段
	method := root.Get("method")
	if method.LastError() != nil {
		return fmt.Errorf("root: method NOT exist")
	}

	// method必须是数组类型
	if method.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("root: method is NOT array")
	}

	// 检查每个状态
	visited := make(map[string]struct{})
	for i := 0; i < state.Size(); i++ {
		if err := checkState(state.Get(i), visited); err != nil {
			return fmt.Errorf("state[%d]: %s", i, err)
		}
	}

	// 检查每个事件
	visited = make(map[string]struct{})
	for i := 0; i < event.Size(); i++ {
		if err := checkEvent(event.Get(i), visited); err != nil {
			return fmt.Errorf("event[%d]: %s", i, err)
		}
	}

	// 检查每个方法
	visited = make(map[string]struct{})
	for i := 0; i < method.Size(); i++ {
		if err := checkMethod(method.Get(i), visited); err != nil {
			return fmt.Errorf("method[%d]: %s", i, err)
		}
	}

	return nil
}

func checkState(state jsoniter.Any, visited map[string]struct{}) error {

	if err := checkParamInfo(state, false); err != nil {
		return err
	}

	// 确保状态名不重复
	stateName := state.Get("name").ToString()
	if _, seen := visited[stateName]; seen {
		return fmt.Errorf("repeat state name: %q", stateName)
	} else {
		visited[stateName] = struct{}{}
	}

	return nil
}

func checkEvent(event jsoniter.Any, visited map[string]struct{}) error {
	// 事件元信息必须是对象
	if event.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("NOT object")
	}

	// 检查name和description字段
	if err := checkNameDesc(event); err != nil {
		return err
	}

	// 事件元信息必须包含args字段
	args := event.Get("args")
	if args.LastError() != nil {
		return fmt.Errorf("NO args")
	}

	// args字段必须是数组类型
	if args.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("args is NOT array")
	}

	argsName := make(map[string]struct{})
	for i := 0; i < args.Size(); i++ {
		// 检查参数本身
		if err := checkParamInfo(args.Get(i), false); err != nil {
			return fmt.Errorf("args[%d]: %s", i, err)
		}

		// 确保事件参数名称不重复
		argName := strings.Trim(args.Get(i).Get("name").ToString(), " \t\n\r\f\v")
		if _, seen := argsName[argName]; seen {
			return fmt.Errorf("args[%d]: repeat arg name: %q", i, argName)
		} else {
			argsName[argName] = struct{}{}
		}
	}

	// 确保事件名不能重复
	eventName := strings.Trim(event.Get("name").ToString(), " \t\n\r\f\v")
	if _, seen := visited[eventName]; seen {
		return fmt.Errorf("repeat event name: %q", eventName)
	} else {
		visited[eventName] = struct{}{}
	}

	return nil
}

func checkMethod(method jsoniter.Any, visited map[string]struct{}) error {
	// 方法元信息必须是对象
	if method.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("NOT object")
	}

	// 检查name和description字段
	if err := checkNameDesc(method); err != nil {
		return err
	}

	// 方法元信息必须包含args字段
	args := method.Get("args")
	if args.LastError() != nil {
		return fmt.Errorf("NO args")
	}

	// args字段必须是数组类型
	if args.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("args is NOT array")
	}

	// 逐个检查每个参数
	argsName := make(map[string]struct{})
	for i := 0; i < args.Size(); i++ {
		// 检查参数本身
		if err := checkParamInfo(args.Get(i), false); err != nil {
			return fmt.Errorf("args[%d]: %s", i, err)
		}

		// 确保方法参数名称不重复
		argName := strings.Trim(args.Get(i).Get("name").ToString(), " \t\n\r\f\v")
		if _, seen := argsName[argName]; seen {
			return fmt.Errorf("args[%d]: repeat arg name: %q", i, argName)
		} else {
			argsName[argName] = struct{}{}
		}
	}

	// 方法元信息必须有response字段
	response := method.Get("response")
	if response.LastError() != nil {
		return fmt.Errorf("NO response")
	}

	// response字段必须是数组类型
	if response.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("response is NOT array")
	}

	// 逐个检查每个返回值
	respNameSet := make(map[string]struct{})
	for i := 0; i < response.Size(); i++ {
		// 检查返回本身
		if err := checkParamInfo(response.Get(i), false); err != nil {
			return fmt.Errorf("response[%d]: %s", i, err)
		}

		// 确保方法返回值名称不重复
		respName := strings.Trim(response.Get(i).Get("name").ToString(), " \t\n\r\f\v")
		if _, seen := respNameSet[respName]; seen {
			return fmt.Errorf("response[%d]: repeat resp name: %q", i, respName)
		} else {
			respNameSet[respName] = struct{}{}
		}
	}

	// 确保事件名不能重复
	methodName := strings.Trim(method.Get("name").ToString(), " \t\n\r\f\v")
	if _, seen := visited[methodName]; seen {
		return fmt.Errorf("repeat method name: %q", methodName)
	} else {
		visited[methodName] = struct{}{}
	}

	return nil
}

func checkNameDesc(obj jsoniter.Any) error {
	// 必须包含name字段
	name := obj.Get("name")
	if name.LastError() != nil {
		return fmt.Errorf("name NOT exist")
	}

	// name字段必须是字符串类型
	if name.ValueType() != jsoniter.StringValue {
		return fmt.Errorf("name is NOT string")
	}

	// name字段不能为空字符串
	if strings.Trim(name.ToString(), " \t\n\r\f\v") == "" {
		return fmt.Errorf("name is empty")
	}

	// 必须包含description字段
	description := obj.Get("description")
	if description.LastError() != nil {
		return fmt.Errorf("description NOT exist")
	}

	// description字段必须是字符串类型
	if description.ValueType() != jsoniter.StringValue {
		return fmt.Errorf("description is NOT string")
	}

	// description字段不能为空字符串
	if strings.Trim(description.ToString(), " \t\n\r\f\v") == "" {
		return fmt.Errorf("description is empty")
	}

	return nil
}

func checkParamInfo(obj jsoniter.Any, isElement bool) error {
	// 元信息必须是对象
	if obj.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("NOT object")
	}

	// 不是element情况下检查name和description字段
	if !isElement {
		if err := checkNameDesc(obj); err != nil {
			return err
		}
	}

	// 状态元信息必须包含type字段
	Type := obj.Get("type")
	if Type.LastError() != nil {
		return fmt.Errorf("type NOT exist")
	}

	// type字段必须是字符串类型
	if Type.ValueType() != jsoniter.StringValue {
		return fmt.Errorf("type is NOT string")
	}

	// type字段值不能为空
	typeStr := strings.Trim(Type.ToString(), " \t\n\r\f\v")
	if typeStr == "" {
		return fmt.Errorf("type is empty")
	}

	// type字段的值必须有效
	if _, seen := validType[typeStr]; !seen {
		return fmt.Errorf("invalid type: %q", typeStr)
	}

	// 根据type字段值进一步检查
	switch typeStr {
	case "array":
		// 数组类型必须有length字段
		length := obj.Get("length")
		if length.LastError() != nil {
			return fmt.Errorf("length NOT exist in array")
		}

		// length字段必须是数值类型
		if length.ValueType() != jsoniter.NumberValue {
			return fmt.Errorf("length is NOT number")
		}

		// length字段必须能转成uint
		lengthVal := length.ToUint()
		if length.LastError() != nil {
			return fmt.Errorf("length is NOT uint")
		}

		// length必须大于0
		if lengthVal <= 0 {
			return fmt.Errorf("length is NOT greater than 0")
		}

		// 数组类型必须有element字段
		element := obj.Get("element")
		if element.LastError() != nil {
			return fmt.Errorf("element NOT exist")
		}

		// 检查element
		if err := checkParamInfo(element, true); err != nil {
			return fmt.Errorf("element: %s", err)
		}
	case "struct":
		// 结构体类型必须有fields字段
		fields := obj.Get("fields")
		if fields.LastError() != nil {
			return fmt.Errorf("fields NOT exist")
		}

		// fields字段必须是数组类型
		if fields.ValueType() != jsoniter.ArrayValue {
			return fmt.Errorf("fields is NOT array")
		}

		fieldSet := make(map[string]struct{})
		for i := 0; i < fields.Size(); i++ {
			// 检查字段本身
			if err := checkParamInfo(fields.Get(i), false); err != nil {
				return fmt.Errorf("fields[%d]: %s", i, err)
			}

			// 确保字段名不重复
			fieldName := fields.Get(i).Get("name").ToString()
			if _, seen := fieldSet[fieldName]; seen {
				return fmt.Errorf("fields[%d]: repeat field name: %q", i, fieldName)
			} else {
				fieldSet[fieldName] = struct{}{}
			}
		}

	case "slice":
		// 切片类型必须有element字段
		element := obj.Get("element")
		if element.LastError() != nil {
			return fmt.Errorf("element NOT exist")
		}

		// 检查element
		if err := checkParamInfo(element, true); err != nil {
			return fmt.Errorf("element: %s", err)
		}
	case "int", "uint", "float":
		unit := obj.Get("unit")

		if unit.LastError() == nil {
			// 在unit字段存在的情况下，必须是字符串类型
			if unit.ValueType() != jsoniter.StringValue {
				return fmt.Errorf("unit is NOT string")
			}
			// unit不能时空字符串
			if strings.Trim(unit.ToString(), " \t\n\r\f\v") == "" {
				return fmt.Errorf("unit is empty")
			}
		}
	}

	// 如果存在range字段，则对range字段值检查
	rangeObj := obj.Get("range")
	if rangeObj.LastError() == nil {
		if err := checkRange(rangeObj, typeStr); err != nil {
			return err
		}
	}

	return nil
}

func checkRange(rangeObj jsoniter.Any, typeStr string) error {
	if rangeObj.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("range: NOT object")
	}

	switch typeStr {
	case "string":
		return checkStringRange(rangeObj)
	case "float":
		return checkFloatRange(rangeObj)
	case "int":
		return checkIntRange(rangeObj)
	case "uint":
		return checkUintRange(rangeObj)
	default:
		return fmt.Errorf("range: %q NOT support range", typeStr)
	}

	return nil
}

func checkStringRange(rangeObj jsoniter.Any) error {
	// string类型的range必须有option字段
	option := rangeObj.Get("option")
	if option.LastError() != nil {
		return fmt.Errorf("range: NO option for string range")
	}

	// option字段必须是数组类型
	if option.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("range: option: NOT array")
	}

	// option必须包含1个以上选项
	if option.Size() < 1 {
		return fmt.Errorf("range: option: size less than 1")
	}

	// 逐个检查每个选项
	valueSet := make(map[string]struct{})
	for i := 0; i < option.Size(); i++ {
		optionItem := option.Get(i)
		// 每个option选项必须是对象
		if optionItem.ValueType() != jsoniter.ObjectValue {
			return fmt.Errorf("range: option[%d]: NOT object", i)
		}

		// 每个option选项必须包含value
		optionValue := optionItem.Get("value")
		if optionValue.LastError() != nil {
			return fmt.Errorf("range: option[%d]: value NOT exist", i)
		}

		// 每个option选项包含的value必须是string类型
		if optionValue.ValueType() != jsoniter.StringValue {
			return fmt.Errorf("range: option[%d]: value is NOT string", i)
		}

		// 每个option选项的value值不能为空
		valueStr := strings.Trim(optionValue.ToString(), " \t\r\n\f\v")
		if valueStr == "" {
			return fmt.Errorf("range: option[%d]: value is empty", i)
		}

		// 每个option选项的value值不能重复
		if _, seen := valueSet[valueStr]; seen {
			return fmt.Errorf("range: option[%d]: repeat value: %q", i, valueStr)
		} else {
			valueSet[valueStr] = struct{}{}
		}

		// 每个option选项必须包含description
		description := optionItem.Get("description")
		if description.LastError() != nil {
			return fmt.Errorf("range: option[%d]: description NOT exist", i)
		}

		// 每个option选项包含的description必须是string类型
		if description.ValueType() != jsoniter.StringValue {
			return fmt.Errorf("range: option[%d]: description is NOT string", i)
		}

		// 每个option选项包含的description不能为空字符串
		if strings.Trim(description.ToString(), " \t\n\r\f\v") == "" {
			return fmt.Errorf("range: option[%d]: description is empty", i)
		}
	}

	// 如果有default字段，检查默认值是否合理
	Default := rangeObj.Get("default")
	if Default.LastError() == nil {
		if Default.ValueType() != jsoniter.StringValue {
			return fmt.Errorf("range: default: NOT string")
		}

		defaultVal := strings.Trim(Default.ToString(), " \t\n\r\f\v")

		// default不能为空字符串
		if defaultVal == "" {
			return fmt.Errorf("range: default is empty")
		}

		if _, seen := valueSet[defaultVal]; !seen {
			return fmt.Errorf("range: default: %q NOT in option", defaultVal)
		}
	}
	return nil
}

func checkFloatRange(rangeObj jsoniter.Any) error {
	minCfg := rangeObj.Get("min")
	maxCfg := rangeObj.Get("max")

	var maxGot bool
	var minGot bool
	var max float64
	var min float64

	maxGot = maxCfg.LastError() == nil
	minGot = minCfg.LastError() == nil

	// float类型的range必须有min 或 max字段, 不能两个都没有
	if !maxGot && !minGot {
		return fmt.Errorf("range: NO min or max for float range")
	}

	// 在有min字段情况下, float类型的min字段必须是double类型
	if minGot {
		if minCfg.ValueType() != jsoniter.NumberValue {
			return fmt.Errorf("range: min: NOT number")
		}
		min = minCfg.ToFloat64()
		if minCfg.LastError() != nil {
			return fmt.Errorf("range: min: NOT float")
		}
	}

	// 在有max字段情况下, float类型的max字段必须是double类型
	if maxGot {
		if maxCfg.ValueType() != jsoniter.NumberValue {
			return fmt.Errorf("range: max: NOT number")
		}
		max = maxCfg.ToFloat64()
		if maxCfg.LastError() != nil {
			return fmt.Errorf("range: max: NOT float")
		}
	}

	// 在max和min字段都存在的情况下，最小值一定严格小于最大值
	if maxGot && minGot {
		if min > max {
			return fmt.Errorf("range: min is NOT less than max")
		}
	}

	// 如果有default字段，检查默认值是否合理
	Default := rangeObj.Get("default")
	if Default.LastError() == nil {

		if Default.ValueType() != jsoniter.NumberValue {
			return fmt.Errorf("range: default: NOT number")
		}

		defaultValue := Default.ToFloat64()

		if Default.LastError() != nil {
			return fmt.Errorf("range: default: NOT float")
		}

		// 默认值必须介于[min, max]之间
		if minGot && defaultValue < min {
			return fmt.Errorf("range: default: less than min")
		}

		if maxGot && defaultValue > max {
			return fmt.Errorf("range: default: greater than max")
		}
	}

	return nil
}

func checkIntRange(rangeObj jsoniter.Any) error {
	option := rangeObj.Get("option")
	// 如果int类型的range有option字段，则以option为准
	// 否则以最大值max、最小值min为准
	if option.LastError() == nil {
		// option字段必须是数组类型
		if option.ValueType() != jsoniter.ArrayValue {
			return fmt.Errorf("range: option: NOT array")
		}
		// option必须包含1个以上选项
		if option.Size() < 1 {
			return fmt.Errorf("range: option: size less than 1")
		}

		valueSet := make(map[int]struct{})
		for i := 0; i < option.Size(); i++ {
			optionItem := option.Get(i)
			// 每个option选项必须是对象
			if optionItem.ValueType() != jsoniter.ObjectValue {
				return fmt.Errorf("range: option[%d]: NOT object", i)
			}

			// 每个option选项必须包含value
			optionValue := optionItem.Get("value")
			if optionValue.LastError() != nil {
				return fmt.Errorf("range: option[%d]: value NOT exist", i)
			}

			// 每个option选项包含的value必须是number类型
			if optionValue.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: option[%d]: value is NOT number", i)
			}

			value := optionValue.ToInt()
			if optionValue.LastError() != nil {
				return fmt.Errorf("range: option[%d]: value is NOT int", i)
			}

			// 每个option选项的value值不能重复
			if _, seen := valueSet[value]; seen {
				return fmt.Errorf("range: option[%d]: repeat value: %d", i, value)
			} else {
				valueSet[value] = struct{}{}
			}

			// 每个option选项必须包含description
			description := optionItem.Get("description")
			if description.LastError() != nil {
				return fmt.Errorf("range: option[%d]: description NOT exist", i)
			}

			// 每个option选项包含的description必须是string类型
			if description.ValueType() != jsoniter.StringValue {
				return fmt.Errorf("range: option[%d]: description is NOT string", i)
			}

			// 每个option选项包含的description不能为空字符串
			if strings.Trim(description.ToString(), " \t\n\r\f\v") == "" {
				return fmt.Errorf("range: option[%d]: description is empty", i)
			}
		}

		// 如果有default字段，检查默认值是否合理
		Default := rangeObj.Get("default")
		if Default.LastError() == nil {
			// 默认值必须是int
			if Default.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: default: NOT number")
			}
			defaultVal := Default.ToInt()
			if Default.LastError() != nil {
				return fmt.Errorf("range: default: NOT int")
			}

			// 默认值必须在可选值列表中
			if _, seen := valueSet[defaultVal]; !seen {
				return fmt.Errorf("range: default: %q NOT in option", defaultVal)
			}
		}
	} else {
		minCfg := rangeObj.Get("min")
		maxCfg := rangeObj.Get("max")

		var maxGot bool
		var minGot bool
		var max int
		var min int

		maxGot = maxCfg.LastError() == nil
		minGot = minCfg.LastError() == nil

		// int类型的range必须有min 或 max字段, 不能两个都没有
		if !maxGot && !minGot {
			return fmt.Errorf("range: NO min and max for int range")
		}

		// 在有min字段情况下, float类型的min字段必须是int类型
		if minGot {
			if minCfg.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: min: NOT number")
			}
			min = minCfg.ToInt()
			if minCfg.LastError() != nil {
				return fmt.Errorf("range: min: NOT int")
			}
		}

		// 在有max字段情况下, float类型的max字段必须是double类型
		if maxGot {
			if maxCfg.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: max: NOT number")
			}
			max = maxCfg.ToInt()
			if maxCfg.LastError() != nil {
				return fmt.Errorf("range: max: NOT int")
			}
		}

		// 在max和min字段都存在的情况下，最小值一定严格小于最大值
		if maxGot && minGot {
			if min > max {
				return fmt.Errorf("range: min is NOT less than max")
			}
		}

		// 如果有default字段，检查默认值是否合理
		Default := rangeObj.Get("default")
		if Default.LastError() == nil {
			if Default.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: default: NOT number")
			}
			defaultValue := Default.ToInt()

			if Default.LastError() != nil {
				return fmt.Errorf("range: default: NOT int")
			}

			// 默认值必须介于[min, max]之间
			if minGot && defaultValue < min {
				return fmt.Errorf("range: default: less than min")
			}

			if maxGot && defaultValue > max {
				return fmt.Errorf("range: default: greater than max")
			}
		}
	}
	return nil
}

func checkUintRange(rangeObj jsoniter.Any) error {
	option := rangeObj.Get("option")
	// 如果uint类型的range有option字段，则以option为准
	// 否则以最大值max、最小值min为准
	if option.LastError() == nil {
		// option字段必须是数组类型
		if option.ValueType() != jsoniter.ArrayValue {
			return fmt.Errorf("range: option: NOT array")
		}

		// option必须包含1个以上选项
		if option.Size() < 1 {
			return fmt.Errorf("range: option: size less than 1")
		}

		valueSet := make(map[uint]struct{})
		for i := 0; i < option.Size(); i++ {
			optionItem := option.Get(i)
			// 每个option选项必须是对象
			if optionItem.ValueType() != jsoniter.ObjectValue {
				return fmt.Errorf("range: option[%d]: NOT object", i)
			}

			// 每个option选项必须包含value
			optionValue := optionItem.Get("value")
			if optionValue.LastError() != nil {
				return fmt.Errorf("range: option[%d]: value NOT exist", i)
			}

			// 每个option选项包含的value必须是number类型
			if optionValue.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: option[%d]: value is NOT number", i)
			}

			value := optionValue.ToUint()
			if optionValue.LastError() != nil {
				return fmt.Errorf("range: option[%d]: value is NOT uint", i)
			}

			// 每个option选项的value值不能重复
			if _, seen := valueSet[value]; seen {
				return fmt.Errorf("range: option[%d]: repeat value: %d", i, value)
			} else {
				valueSet[value] = struct{}{}
			}

			// 每个option选项必须包含description
			description := optionItem.Get("description")
			if description.LastError() != nil {
				return fmt.Errorf("range: option[%d]: description NOT exist", i)
			}

			// 每个option选项包含的description必须是string类型
			if description.ValueType() != jsoniter.StringValue {
				return fmt.Errorf("range: option[%d]: description is NOT string", i)
			}

			// 每个option选项包含的description不能为空字符串
			if strings.Trim(description.ToString(), " \t\n\r\f\v") == "" {
				return fmt.Errorf("range: option[%d]: description is empty", i)
			}
		}

		// 如果有default字段，检查默认值是否合理
		Default := rangeObj.Get("default")
		if Default.LastError() == nil {
			// 默认值必须是int
			if Default.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: default: NOT number")
			}
			defaultVal := Default.ToUint()
			if Default.LastError() != nil {
				return fmt.Errorf("range: default: NOT uint")
			}

			// 默认值必须在可选值列表中
			if _, seen := valueSet[defaultVal]; !seen {
				return fmt.Errorf("range: default: %d NOT in option", defaultVal)
			}
		}
	} else {
		minCfg := rangeObj.Get("min")
		maxCfg := rangeObj.Get("max")

		var maxGot bool
		var minGot bool
		var max uint
		var min uint

		maxGot = maxCfg.LastError() == nil
		minGot = minCfg.LastError() == nil

		// int类型的range必须有min 或 max字段, 不能两个都没有
		if !maxGot && !minGot {
			return fmt.Errorf("range: NO min or max for uint range")
		}

		// 在有min字段情况下, float类型的min字段必须是int类型
		if minGot {
			if minCfg.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: min: NOT number")
			}
			min = minCfg.ToUint()
			if minCfg.LastError() != nil {
				return fmt.Errorf("range: min: NOT uint")
			}
		}

		// 在有max字段情况下, float类型的max字段必须是double类型
		if maxGot {
			if maxCfg.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: max: NOT number")
			}
			max = maxCfg.ToUint()
			if maxCfg.LastError() != nil {
				return fmt.Errorf("range: max: NOT uint")
			}
		}

		// 在max和min字段都存在的情况下，最小值一定严格小于最大值
		if maxGot && minGot {
			if min > max {
				return fmt.Errorf("range: min is NOT less than max")
			}
		}

		// 如果有default字段，检查默认值是否合理
		Default := rangeObj.Get("default")
		if Default.LastError() == nil {
			if Default.ValueType() != jsoniter.NumberValue {
				return fmt.Errorf("range: default: NOT number")
			}
			defaultValue := Default.ToUint()

			if Default.LastError() != nil {
				return fmt.Errorf("range: default: NOT uint")
			}

			// 默认值必须介于[min, max]之间
			if minGot && defaultValue < min {
				return fmt.Errorf("range: default: less than min")
			}

			if maxGot && defaultValue > max {
				return fmt.Errorf("range: default: greater than max")
			}
		}
	}
	return nil
}

func checkModelName(name string) error {
	// 1.先以/分割
	tokens := strings.Split(name, "/")

	// 2.去除每个token前后空格
	for i, token := range tokens {
		tokens[i] = strings.Trim(token, " \t\n\r\f\v")
	}

	// 3.过滤空的token
	trimmedToken := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != "" {
			trimmedToken = append(trimmedToken, token)
		}
	}

	// 4.规范化后tokens不能为空
	if len(trimmedToken) == 0 {
		return fmt.Errorf("empty model name after normalize")
	}

	// 5.检查模板
	visited := make(map[string]struct{})
	for _, token := range trimmedToken {
		// 没有以{开头, 但以}结尾
		if !strings.HasPrefix(token, "{") && strings.HasSuffix(token, "}") {
			return fmt.Errorf("template %q: missing '{'", token)
		}

		// 以{开头, 但没有以}结尾
		if strings.HasPrefix(token, "{") && !strings.HasSuffix(token, "}") {
			return fmt.Errorf("template %q: missing '}'", token)
		}

		// {...} 形式的模板
		if strings.HasPrefix(token, "{") && strings.HasSuffix(token, "}") {
			// 去除{和 } 并 去除空格后的 模板名称
			templateName := strings.Trim(token[1:len(token)-1], " \t\n\r\f\v")

			// 模板名称不允许为空
			if templateName == "" {
				return fmt.Errorf("empty template name")
			}

			// 不允许再由多余的{
			if strings.Contains(templateName, "{") {
				return fmt.Errorf("template %q: contains extra '{'", templateName)
			}

			// 不允许再由多余的}
			if strings.Contains(templateName, "}") {
				return fmt.Errorf("template %q: contains extra '}'", templateName)
			}

			// 模板名称不能重复
			if _, seen := visited[templateName]; seen {
				return fmt.Errorf("repeat template: %q", templateName)
			} else {
				visited[templateName] = struct{}{}
			}
		}
	}

	return nil
}

func createParamMeta(param jsoniter.Any) ParamMeta {
	ans := ParamMeta{
		Type: strings.TrimSpace(param.Get("type").ToString()),
	}

	name := param.Get("name")
	if name.LastError() == nil {
		nameStr := strings.TrimSpace(name.ToString())
		ans.Name = &nameStr
	}

	description := param.Get("description")
	if description.LastError() == nil {
		descriptionStr := strings.TrimSpace(description.ToString())
		ans.Description = &descriptionStr
	}

	element := param.Get("element")
	if element.LastError() == nil {
		eleMeta := createParamMeta(element)
		ans.Element = &eleMeta
	}

	fields := param.Get("fields")
	if fields.LastError() == nil {
		ans.Fields = make([]ParamMeta, 0, fields.Size())
		for i := 0; i < fields.Size(); i++ {
			ans.Fields = append(ans.Fields, createParamMeta(fields.Get(i)))
		}
	}

	length := param.Get("length")
	if length.LastError() == nil {
		lengthVal := length.ToUint()
		ans.Length = &lengthVal
	}

	unit := param.Get("unit")
	if unit.LastError() == nil {
		unitVal := strings.TrimSpace(unit.ToString())
		ans.Unit = &unitVal
	}

	rangeObj := param.Get("range")
	if rangeObj.LastError() == nil {
		ans.Range = &RangeInfo{}
		minCfg := rangeObj.Get("min")
		if minCfg.LastError() == nil {
			ans.Range.Min = getVal(ans.Type, minCfg)
		}
		maxCfg := rangeObj.Get("max")
		if maxCfg.LastError() == nil {
			ans.Range.Max = getVal(ans.Type, maxCfg)
		}
		optionCfg := rangeObj.Get("option")
		if optionCfg.LastError() == nil {
			ans.Range.Option = make([]OptionInfo, 0, optionCfg.Size())
			for i := 0; i < optionCfg.Size(); i++ {
				ans.Range.Option = append(ans.Range.Option, OptionInfo{
					Value:       getVal(ans.Type, optionCfg.Get(i).Get("value")),
					Description: strings.TrimSpace(optionCfg.Get(i).Get("description").ToString()),
				})
			}
		}
		defaultCfg := rangeObj.Get("default")
		if defaultCfg.LastError() == nil {
			ans.Range.Default = getVal(ans.Type, defaultCfg)
		}
	}
	return ans
}

func createEventMeta(event jsoniter.Any) EventMeta {
	ans := EventMeta{
		Name:        strings.TrimSpace(event.Get("name").ToString()),
		Description: strings.TrimSpace(event.Get("description").ToString()),
		Args:        make([]ParamMeta, 0, event.Get("args").Size()),
	}

	for i := 0; i < event.Get("args").Size(); i++ {
		ans.Args = append(ans.Args, createParamMeta(event.Get("args").Get(i)))
	}

	return ans
}

func createMethodMeta(method jsoniter.Any) MethodMeta {
	ans := MethodMeta{
		Name:        strings.TrimSpace(method.Get("name").ToString()),
		Description: strings.TrimSpace(method.Get("description").ToString()),
		Args:        make([]ParamMeta, 0, method.Get("args").Size()),
		Response:    make([]ParamMeta, 0, method.Get("response").Size()),
	}

	for i := 0; i < method.Get("args").Size(); i++ {
		ans.Args = append(ans.Args, createParamMeta(method.Get("args").Get(i)))
	}

	for i := 0; i < method.Get("response").Size(); i++ {
		ans.Response = append(ans.Response, createParamMeta(method.Get("response").Get(i)))
	}

	return ans
}

func getVal(Type string, any jsoniter.Any) interface{} {
	switch Type {
	case "int":
		return any.ToInt()
	case "uint":
		return any.ToUint()
	case "float":
		return any.ToFloat64()
	case "string":
		return strings.TrimSpace(any.ToString())
	default:
		return nil
	}
}

func trimTemplate(param TemplateParam) TemplateParam {
	ans := make(map[string]string)
	for name, val := range param {
		ans[strings.TrimSpace(name)] = strings.TrimSpace(val)
	}
	return ans
}
