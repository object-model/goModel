package meta

import (
	"fmt"
	"github.com/google/uuid"
	jsoniter "github.com/json-iterator/go"
	"github.com/object-model/goModel/message"
	"reflect"
	"strings"
	"sync"
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

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// OptionInfo 为选项元信息
type OptionInfo struct {
	Value       interface{} `json:"value"`       // 选项值
	Description string      `json:"description"` // 选项描述
}

// RangeInfo 为范围约束元信息
type RangeInfo struct {
	Max     interface{}  `json:"max,omitempty"`     // 最大值
	Min     interface{}  `json:"min,omitempty"`     // 最小值
	Option  []OptionInfo `json:"option,omitempty"`  // 可选项
	Default interface{}  `json:"default,omitempty"` // 默认值
}

// ParamMeta 为参数元信息
type ParamMeta struct {
	Name        *string     `json:"name,omitempty"`        // 参数名
	Description *string     `json:"description,omitempty"` // 参数描述
	Type        string      `json:"type"`                  // 参数类型
	Element     *ParamMeta  `json:"element,omitempty"`     // 数组或者切片元素的元信息, 仅在 Type 为数组或切片时有效
	Fields      []ParamMeta `json:"fields,omitempty"`      // 结构体类型参数的字段元信息, 仅在 Type 为结构体时有效
	Length      *uint       `json:"length,omitempty"`      // 数组长度, 仅在 Type 为 数组时有效
	Unit        *string     `json:"unit,omitempty"`        // 参数单位
	Range       *RangeInfo  `json:"range,omitempty"`       // 参数范围, 仅在 Type 为 int uint float string时有效
}

// EventMeta 为事件元信息
type EventMeta struct {
	Name        string      `json:"name"`        // 事件名称
	Description string      `json:"description"` // 事件描述
	Args        []ParamMeta `json:"args"`        // 事件参数
}

// MethodMeta 为方法元信息
type MethodMeta struct {
	Name        string      `json:"name"`        // 方法名称
	Description string      `json:"description"` // 方法描述
	Args        []ParamMeta `json:"args"`        // 方法参数
	Response    []ParamMeta `json:"response"`    // 方法响应
}

// Meta 为物模型元信息
type Meta struct {
	Name        string       `json:"name"`        // 物模型名称
	Description string       `json:"description"` // 物模型描述
	State       []ParamMeta  `json:"state"`       // 状态元信息
	Event       []EventMeta  `json:"event"`       // 事件元信息
	Method      []MethodMeta `json:"method"`      // 方法元信息

	nameTokens    []string       // 物模型名称以/分割后的有效token
	nameTemplates map[string]int // 模板参数名到nameTokens中的索引
	stateIndex    map[string]int // 状态名称索引
	eventIndex    map[string]int // 事件名称索引
	methodIndex   map[string]int // 方法名称索引

	json       []byte    // 缓存元信息序列化后的JSON串
	encodeOnce sync.Once // 只序列化一次
}

// TemplateParam 为元信息模板参数
type TemplateParam map[string]string

// AllStates 返回物模型元信息m中的所有状态全名.
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

// AllEvents 返回物模型元信息m中的所有事件全名.
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

// AllMethods 返回物模型元信息m中的所有方法全名.
func (m *Meta) AllMethods() []string {
	res := make([]string, 0, len(m.Method))
	for i := range m.Method {
		res = append(res, strings.Join([]string{
			m.Name,
			m.Method[i].Name,
		}, "/"))
	}
	return res
}

// ToJSON 将物模型元信息m序列化JSON串.
func (m *Meta) ToJSON() []byte {
	m.encodeOnce.Do(func() {
		data, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}
		m.json = data
	})
	return m.json
}

// VerifyState 验证名称为name数据为data的状态是否符合元信息m, 如果符合返回nil, 如果不符合返回错误信息.
func (m *Meta) VerifyState(name string, data interface{}) error {
	if index, seen := m.stateIndex[name]; !seen {
		return fmt.Errorf("NO state %q", name)
	} else {
		return verifyData(m.State[index], data)
	}
}

// VerifyEvent 验证名为name参数为args的事件是否符合元信息m, 如果符合返回nil, 如果不符合返回错误信息.
func (m *Meta) VerifyEvent(name string, args message.Args) error {
	index, seen := m.eventIndex[name]
	if !seen {
		return fmt.Errorf("NO event %q", name)
	}

	// 参数不能为空
	if args == nil {
		return fmt.Errorf("nil event args")
	}

	// 每个参数是否匹配
	// NOTE: 元信息中每个参数一定要在args中存在，且字段值能匹配
	// NOTE: args中多余的字段不判断, 保持一定的兼容能力
	for _, argMeta := range m.Event[index].Args {
		argName := *argMeta.Name

		// a.参数存在性
		value, seen := args[argName]
		if !seen {
			return fmt.Errorf("arg %q: missing", argName)
		}

		// b.参数值一致性
		if err := verifyData(argMeta, value); err != nil {
			return fmt.Errorf("arg %q: %s", argName, err)
		}
	}

	return nil
}

// VerifyMethodArgs 验证名为name参数为args的调用请求是否符合元信息m, 如果符合返回nil, 如果不符合返回错误信息.
func (m *Meta) VerifyMethodArgs(name string, args message.Args) error {
	index, seen := m.methodIndex[name]
	if !seen {
		return fmt.Errorf("NO method %q", name)
	}

	// 参数不能为空
	if args == nil {
		return fmt.Errorf("nil method args")
	}

	// 2.每个参数是否匹配
	// NOTE: 元信息中每个参数一定要在args中存在，且字段值能匹配
	// NOTE: args中多余的字段不判断, 保持一定的兼容能力
	for _, argMeta := range m.Method[index].Args {
		argName := *argMeta.Name

		// a.参数存在性
		value, seen := args[argName]
		if !seen {
			return fmt.Errorf("arg %q: missing", argName)
		}

		// b.参数值一致性
		if err := verifyData(argMeta, value); err != nil {
			return fmt.Errorf("arg %q: %s", argName, err)
		}
	}

	return nil
}

// VerifyMethodResp 验证名为name返回值为resp的调用响应是否符合元信息m, 如果符合返回nil, 如果不符合返回错误信息.
func (m *Meta) VerifyMethodResp(name string, resp message.Resp) error {
	index, seen := m.methodIndex[name]
	if !seen {
		return fmt.Errorf("NO method %q", name)
	}

	// 返回值不能为空
	if resp == nil {
		return fmt.Errorf("nil method response")
	}

	// 2.每个返回值是否匹配
	// NOTE: 元信息中每个参数一定要在resp中存在，且字段值能匹配
	// NOTE: resp中多余的字段不判断, 保持一定的兼容能力
	for _, respMeta := range m.Method[index].Response {
		respName := *respMeta.Name

		// a.参数存在性
		value, seen := resp[respName]
		if !seen {
			return fmt.Errorf("response %q: missing", respName)
		}

		// b.参数值一致性
		if err := verifyData(respMeta, value); err != nil {
			return fmt.Errorf("response %q: %s", respName, err)
		}
	}

	return nil
}

func verifyData(meta ParamMeta, data interface{}) error {
	return _verifyData_(meta, data, true)
}

func _verifyData_(meta ParamMeta, data interface{}, checkRange bool) error {
	if data == nil {
		return fmt.Errorf("nil")
	}
	switch meta.Type {
	case "int":
		return verifyIntData(meta, data, checkRange)
	case "uint":
		return verifyUintData(meta, data, checkRange)
	case "float":
		return verifyFloatData(meta, data, checkRange)
	case "bool":
		if _, isBool := data.(bool); !isBool {
			return fmt.Errorf("type unmatched")
		}
	case "string":
		return verifyStringData(meta, data, checkRange)
	case "array":
		return verifyArrayData(meta, data, checkRange)
	case "slice":
		return verifySliceData(meta, data, checkRange)
	case "struct":
		return verifyStructData(meta, data, checkRange)
	case "meta":
		return verifyMetaData(data)
	}
	return nil
}

func verifyIntData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	var value int
	switch data.(type) {
	case int:
		value = data.(int)
	case int8:
		value = int(data.(int8))
	case int16:
		value = int(data.(int16))
	case int32:
		value = int(data.(int32))
	case int64:
		value = int(data.(int64))
	default:
		return fmt.Errorf("type unmatched")
	}

	// 2.如果有范围约束，检查是否在范围内
	if checkRange {
		return verifyRangeForInt(meta.Range, value)
	}

	return nil
}

func verifyUintData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	var value uint
	switch data.(type) {
	case uint:
		value = data.(uint)
	case uint8:
		value = uint(data.(uint8))
	case uint16:
		value = uint(data.(uint16))
	case uint32:
		value = uint(data.(uint32))
	case uint64:
		value = uint(data.(uint64))
	default:
		return fmt.Errorf("type unmatched")
	}

	// 2.如果有范围约束，检查是否在范围内
	if checkRange {
		return verifyRangeForUint(meta.Range, value)
	}

	return nil
}

func verifyFloatData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	var value float64
	switch data.(type) {
	case float64:
		value = data.(float64)
	case float32:
		value = float64(data.(float32))
	case int:
		value = float64(data.(int))
	case int8:
		value = float64(data.(int8))
	case int16:
		value = float64(data.(int16))
	case int32:
		value = float64(data.(int32))
	case int64:
		value = float64(data.(int64))
	case uint:
		value = float64(data.(uint))
	case uint8:
		value = float64(data.(uint8))
	case uint16:
		value = float64(data.(uint16))
	case uint32:
		value = float64(data.(uint32))
	case uint64:
		value = float64(data.(uint64))
	default:
		return fmt.Errorf("type unmatched")
	}

	// 2.如果有范围约束，检查是否在范围内
	if checkRange {
		return verifyRangeForFloat(meta.Range, value)
	}

	return nil
}

func verifyStringData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	value, isString := data.(string)
	if !isString {
		return fmt.Errorf("type unmatched")
	}

	// 2.如果有范围约束，检查是否在范围内
	if checkRange {
		return verifyRangeForString(meta.Range, value)
	}
	return nil
}

func verifyArrayData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	if reflect.TypeOf(data).Kind() != reflect.Array {
		return fmt.Errorf("type unmatched")
	}

	// 2.长度匹配
	if uint(reflect.TypeOf(data).Len()) != *meta.Length {
		return fmt.Errorf("length NOT equal to %d", *meta.Length)
	}

	// 3.数组元素类型也得匹配
	// NOTE: 必须要先判断数组元素类型是否匹配
	// NOTE: 另外，在检查数组元素类型时不检查范围，避免因范围不通过而导致的类型错误
	zeroElem := reflect.New(reflect.TypeOf(data).Elem()).Elem().Interface()
	if err := _verifyData_(*meta.Element, zeroElem, false); err != nil {
		return fmt.Errorf("element: %s", err)
	}

	// 4.数组中每个元素是否匹配
	value := reflect.ValueOf(data)
	for i := 0; i < value.Len(); i++ {
		err := _verifyData_(*meta.Element, value.Index(i).Interface(), checkRange)
		if err != nil {
			return fmt.Errorf("element[%d]: %s", i, err)
		}
	}
	return nil
}

func verifySliceData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	kind := reflect.TypeOf(data).Kind()
	if kind != reflect.Array && kind != reflect.Slice {
		return fmt.Errorf("type unmatched")
	}

	// 2.切片元素类型也得匹配
	// NOTE: 必须要先判断切片元素类型是否匹配！
	// NOTE: 否则在传入一个空的切片但元素类型不匹配时，会因为进入不了步骤4的判断，而导致校验通过！
	// NOTE: 另外，在检查切片元素类型时不检查范围，避免因范围不通过而导致的类型错误
	zeroElem := reflect.New(reflect.TypeOf(data).Elem()).Elem().Interface()
	if err := _verifyData_(*meta.Element, zeroElem, false); err != nil {
		return fmt.Errorf("element: %s", err)
	}

	// 3.不能是nil的切片，但可以是长度为0的切片
	value := reflect.ValueOf(data)
	if kind == reflect.Slice && value.IsNil() {
		return fmt.Errorf("nil slice")
	}

	// 4.切片中每个元素是否匹配
	for i := 0; i < value.Len(); i++ {
		err := _verifyData_(*meta.Element, value.Index(i).Interface(), checkRange)
		if err != nil {
			return fmt.Errorf("element[%d]: %s", i, err)
		}
	}
	return nil
}

// TODO: 后续是否可以支持map[string]类型
func verifyStructData(meta ParamMeta, data interface{}, checkRange bool) error {
	// 1.类型是否匹配
	Type := reflect.TypeOf(data)
	kind := reflect.TypeOf(data).Kind()
	if kind != reflect.Struct {
		return fmt.Errorf("type unmatched")
	}

	// 2.每个成员是否匹配
	value := reflect.ValueOf(data)
	for i := range meta.Fields {
		fieldName := *(meta.Fields[i].Name)

		var fieldType reflect.StructField
		var found bool = false

		// 查找json标签为fieldName的字段类型
		for j := 0; j < Type.NumField(); j++ {
			if tag, ok := Type.Field(j).Tag.Lookup("json"); ok {
				if tag == fieldName {
					fieldType = Type.Field(j)
					found = true
					break
				}
			}
		}

		if found {
			if fieldType.PkgPath != "" {
				return fmt.Errorf("field %q: unexported", fieldName)
			}
		} else {
			return fmt.Errorf("field %q: missing", fieldName)
		}

		fieldValue := value.FieldByName(fieldType.Name)

		if err := _verifyData_(meta.Fields[i], fieldValue.Interface(), checkRange); err != nil {
			return fmt.Errorf("field %q: %s", fieldName, err)
		}
	}
	return nil
}

func verifyMetaData(data interface{}) error {
	meta, isMeta := data.(Meta)
	if !isMeta {
		return fmt.Errorf("type unmatched")
	}

	_, err := Parse(meta.ToJSON(), nil)
	return err
}

// VerifyRawState 校验名为name状态原始数据为data的状态是否符合元信息m, 如果不符合返回错误原因.
// VerifyRawState 与 VerifyState 的区别是:
// VerifyRawState 中的data为尚未解析的JSON原始数据, 而 VerifyState 中的data为真实数据，后续需要序列化.
// VerifyRawState 一般用于校验从网络上接收的状态报文是否符合元信息,
// VerifyState 一般用于推送状态前校验待推送的状态是否符合元信息.
func (m *Meta) VerifyRawState(name string, data []byte) error {
	if index, seen := m.stateIndex[name]; !seen {
		return fmt.Errorf("NO state %q", name)
	} else {
		return verifyRawData(m.State[index], data)
	}
}

// VerifyRawEvent 校验名为name事件原始参数为args的事件是否符合元信息m, 如果不符合返回错误原因.
// VerifyRawEvent 一般用于校验从网络上接收的事件报文是否符合元信息,
// VerifyEvent 一般用于推送事件前校验待推送的状态是否符合元信息.
func (m *Meta) VerifyRawEvent(name string, args message.RawArgs) error {
	// 1.事件存在性
	index, seen := m.eventIndex[name]
	if !seen {
		return fmt.Errorf("NO event %q", name)
	}

	// 2.每个参数是否匹配
	// NOTE: 元信息中每个参数一定要在args中存在，且字段值能匹配
	// NOTE: args中多余的字段不判断, 保持一定的兼容能力
	for _, argMeta := range m.Event[index].Args {
		// a.参数存在性
		argName := *argMeta.Name
		arg, seen := args[argName]
		if !seen {
			return fmt.Errorf("arg %q: missing", argName)
		}

		// b.参数一致性
		if err := verifyRawData(argMeta, arg); err != nil {
			return fmt.Errorf("arg %q: %s", argName, err)
		}

	}
	return nil
}

// VerifyRawMethodArgs 校验名为name调用请求原始参数为args的调用请求是否符合元信息m, 如果不符合返回错误原因.
// VerifyRawMethodArgs 一般用于校验从网络上接收的调用请求报文是否符合元信息,
// VerifyMethodArgs 一般用于发送调用请求前校验待发送的调用请求是否符合元信息.
func (m *Meta) VerifyRawMethodArgs(name string, args message.RawArgs) error {
	// 1.方法存在性
	index, seen := m.methodIndex[name]
	if !seen {
		return fmt.Errorf("NO method %q", name)
	}

	// 2.每个参数是否匹配
	// NOTE: 元信息中每个参数一定要在args中存在，且字段值能匹配
	// NOTE: args中多余的字段不判断, 保持一定的兼容能力
	for _, argMeta := range m.Method[index].Args {
		// a.参数存在性
		argName := *argMeta.Name
		arg, seen := args[argName]
		if !seen {
			return fmt.Errorf("arg %q: missing", argName)
		}

		// b.参数一致性
		if err := verifyRawData(argMeta, arg); err != nil {
			return fmt.Errorf("arg %q: %s", argName, err)
		}
	}
	return nil
}

// VerifyRawMethodResp 校验名为name调用响应原始返回值为response的调用响应是否符合元信息m, 如果不符合返回错误原因.
// VerifyRawMethodResp 一般用于校验从网络上接收的调用响应报文是否符合元信息,
// VerifyMethodResp 一般用于发送调用响应前校验待发送的调用响应是否符合元信息.
func (m *Meta) VerifyRawMethodResp(name string, response message.RawResp) error {
	// 1.方法存在性
	index, seen := m.methodIndex[name]
	if !seen {
		return fmt.Errorf("NO method %q", name)
	}

	// 2.每个返回是否匹配
	// NOTE: 元信息中每个返回值一定要在resp中存在，且字段值能匹配
	// NOTE: resp中多余的字段不判断, 保持一定的兼容能力
	for _, respMeta := range m.Method[index].Response {
		// a.参数存在性
		respName := *respMeta.Name
		resp, seen := response[respName]
		if !seen {
			return fmt.Errorf("response %q: missing", respName)
		}

		// b.返回值一致性
		if err := verifyRawData(respMeta, resp); err != nil {
			return fmt.Errorf("response %q: %s", respName, err)
		}
	}
	return nil
}

func verifyRawData(meta ParamMeta, data []byte) error {
	// data必须是有效的JSON数据
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON data")
	}
	it := jsoniter.ParseBytes(json, data)
	root := it.ReadAny()

	return _verifyRawData_(meta, root)
}

func _verifyRawData_(meta ParamMeta, root jsoniter.Any) error {
	switch meta.Type {
	case "int":
		return verifyRawIntData(meta, root)
	case "uint":
		return verifyRawUintData(meta, root)
	case "float":
		return verifyRawFloatData(meta, root)
	case "bool":
		return verifyRawBoolData(root)
	case "string":
		return verifyRawStringData(meta, root)
	case "array":
		return verifyRawArrayData(meta, root)
	case "slice":
		return verifyRawSliceData(meta, root)
	case "struct":
		return verifyRawStructData(meta, root)
	case "meta":
		return verifyRawMetaData(root)
	}
	return nil
}

func verifyRawIntData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是数值类型
	if root.ValueType() != jsoniter.NumberValue {
		return fmt.Errorf("NOT number")
	}

	// 2.必须能转换成int类型
	value := root.ToInt()
	if root.LastError() != nil {
		return fmt.Errorf("NOT int")
	}

	// 3.检查范围约束
	return verifyRangeForInt(meta.Range, value)
}

func verifyRawUintData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是数值类型
	if root.ValueType() != jsoniter.NumberValue {
		return fmt.Errorf("NOT number")
	}

	// 2.必须能转换成uint类型
	value := root.ToUint()
	if root.LastError() != nil {
		return fmt.Errorf("NOT uint")
	}

	// 3.则检查范围
	return verifyRangeForUint(meta.Range, value)
}

func verifyRawFloatData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是数值类型
	if root.ValueType() != jsoniter.NumberValue {
		return fmt.Errorf("NOT number")
	}

	// 2.必须能转换成float64类型
	value := root.ToFloat64()
	if root.LastError() != nil {
		return fmt.Errorf("NOT float")
	}

	// 3.检查范围
	return verifyRangeForFloat(meta.Range, value)
}

func verifyRawBoolData(root jsoniter.Any) error {
	// 1.必须是bool类型
	if root.ValueType() != jsoniter.BoolValue {
		return fmt.Errorf("NOT bool")
	}

	return nil
}

func verifyRawStringData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是string类型
	if root.ValueType() != jsoniter.StringValue {
		return fmt.Errorf("NOT string")
	}

	// 2.检查范围
	return verifyRangeForString(meta.Range, root.ToString())
}

func verifyRawArrayData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是array类型
	if root.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("NOT array")
	}

	// 2.长度必须匹配
	length := *meta.Length
	if uint(root.Size()) != length {
		return fmt.Errorf("length NOT equal to %d", length)
	}

	// 3.逐个比较每个数值元素
	for i := 0; i < root.Size(); i++ {
		if err := _verifyRawData_(*meta.Element, root.Get(i)); err != nil {
			return fmt.Errorf("element[%d]: %s", i, err)
		}
	}

	return nil
}

func verifyRawSliceData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是array类型
	if root.ValueType() != jsoniter.ArrayValue {
		return fmt.Errorf("NOT slice")
	}

	// 2.逐个比较每个数值元素
	for i := 0; i < root.Size(); i++ {
		if err := _verifyRawData_(*meta.Element, root.Get(i)); err != nil {
			return fmt.Errorf("element[%d]: %s", i, err)
		}
	}

	return nil
}

func verifyRawStructData(meta ParamMeta, root jsoniter.Any) error {
	// 1.必须是object类型
	if root.ValueType() != jsoniter.ObjectValue {
		return fmt.Errorf("NOT struct")
	}

	// 2.每个成员是否匹配
	// NOTE: 元信息中每个字段一定要在数据中存在，且字段值能匹配
	// NOTE: 数据中多余的字段不判断，保持一定的兼容性
	for _, fieldMeta := range meta.Fields {
		// a.元信息中的字段一定要在数据中存在
		filedName := *fieldMeta.Name

		field := root.Get(filedName)
		if field.LastError() != nil {
			return fmt.Errorf("field %q: missing", filedName)
		}

		// b.字段值也要匹配
		if err := _verifyRawData_(fieldMeta, field); err != nil {
			return fmt.Errorf("field %q: %s", filedName, err)
		}
	}

	return nil
}

func verifyRawMetaData(root jsoniter.Any) error {
	return check(root)
}

func verifyRangeForInt(rangeInfo *RangeInfo, value int) error {
	// 没有范围约束，无错误
	if rangeInfo == nil {
		return nil
	}

	// 如果有option, 则以option为准，否则以最大最小值为准
	if rangeInfo.Option != nil {
		for _, option := range rangeInfo.Option {
			if option.Value.(int) == value {
				return nil
			}
		}
		return fmt.Errorf("%d NOT in option", value)
	} else {
		if rangeInfo.Min != nil {
			min := rangeInfo.Min.(int)
			if value < min {
				return fmt.Errorf("less than min")
			}
		}
		if rangeInfo.Max != nil {
			max := rangeInfo.Max.(int)
			if value > max {
				return fmt.Errorf("greater than max")
			}
		}
	}
	return nil
}

func verifyRangeForUint(rangeInfo *RangeInfo, value uint) error {
	// 没有范围约束，无错误
	if rangeInfo == nil {
		return nil
	}

	// 如果有option, 则以option为准，否则以最大最小值为准
	if rangeInfo.Option != nil {
		for _, option := range rangeInfo.Option {
			if option.Value.(uint) == value {
				return nil
			}
		}
		return fmt.Errorf("%d NOT in option", value)
	} else {
		if rangeInfo.Min != nil {
			min := rangeInfo.Min.(uint)
			if value < min {
				return fmt.Errorf("less than min")
			}
		}
		if rangeInfo.Max != nil {
			max := rangeInfo.Max.(uint)
			if value > max {
				return fmt.Errorf("greater than max")
			}
		}
	}

	return nil
}

func verifyRangeForFloat(rangeInfo *RangeInfo, value float64) error {
	// 没有范围约束，无错误
	if rangeInfo == nil {
		return nil
	}

	if rangeInfo.Min != nil {
		min := rangeInfo.Min.(float64)
		if value < min {
			return fmt.Errorf("less than min")
		}
	}
	if rangeInfo.Max != nil {
		max := rangeInfo.Max.(float64)
		if value > max {
			return fmt.Errorf("greater than max")
		}
	}

	return nil
}

func verifyRangeForString(rangeInfo *RangeInfo, value string) error {
	// 没有范围约束，无错误
	if rangeInfo == nil {
		return nil
	}

	for _, option := range rangeInfo.Option {
		if option.Value.(string) == value {
			return nil
		}
	}

	return fmt.Errorf("%q NOT in option", value)
}

func (m *Meta) parseTemplate(name string) {
	// 1.先以/分割
	tokens := strings.Split(name, "/")

	// 2.去除每个token前后的空格
	for i, token := range tokens {
		tokens[i] = strings.TrimSpace(token)
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

// Parse 从JSON数据rawData中解析物模型元信息并根据templateParam设置名称模板, 返回解析的元信息和错误.
//
// 如果解析和设置模板中出错, Parse 返回的元信息为通过调用函数 NewEmptyMeta() 返回的空元信息, '
// Parse 保证不会返回值为nil的元信息.
func Parse(rawData []byte, templateParam TemplateParam) (*Meta, error) {
	// 1. 解析JSON数据
	var value interface{}
	if err := json.Unmarshal(rawData, &value); err != nil {
		return NewEmptyMeta(), fmt.Errorf("parse JSON failed")
	}
	it := jsoniter.ParseBytes(json, rawData)
	root := it.ReadAny()

	// 2. 检查元信息是否正确
	if err := check(root); err != nil {
		return NewEmptyMeta(), err
	}

	// 3. 解析
	ans := Meta{
		Description: strings.TrimSpace(root.Get("description").ToString()),
		State:       make([]ParamMeta, 0, root.Get("state").Size()),
		Event:       make([]EventMeta, 0, root.Get("event").Size()),
		Method:      make([]MethodMeta, 0, root.Get("method").Size()),
		stateIndex:  make(map[string]int),
		eventIndex:  make(map[string]int),
		methodIndex: make(map[string]int),
	}

	// 4.解析模板参数
	ans.parseTemplate(root.Get("name").ToString())

	// 3.规范化模板参数
	templateParam = trimTemplate(templateParam)

	// 5.保存模板参数
	if err := ans.setTemplate(templateParam); err != nil {
		return NewEmptyMeta(), err
	}

	// 6.更新模型名称
	ans.Name = strings.Join(ans.nameTokens, "/")

	// 7.解析状态元信息
	for i := 0; i < root.Get("state").Size(); i++ {
		stateMeta := createParamMeta(root.Get("state").Get(i))
		ans.stateIndex[*stateMeta.Name] = i
		ans.State = append(ans.State, stateMeta)
	}

	// 8.解析事件元信息
	for i := 0; i < root.Get("event").Size(); i++ {
		eventMeta := createEventMeta(root.Get("event").Get(i))
		ans.eventIndex[eventMeta.Name] = i
		ans.Event = append(ans.Event, eventMeta)
	}

	// 9.解析方法元信息
	for i := 0; i < root.Get("method").Size(); i++ {
		methodMeta := createMethodMeta(root.Get("method").Get(i))
		ans.methodIndex[methodMeta.Name] = i
		ans.Method = append(ans.Method, methodMeta)
	}

	return &ans, nil
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
		argName := strings.TrimSpace(args.Get(i).Get("name").ToString())
		if _, seen := argsName[argName]; seen {
			return fmt.Errorf("args[%d]: repeat arg name: %q", i, argName)
		} else {
			argsName[argName] = struct{}{}
		}
	}

	// 确保事件名不能重复
	eventName := strings.TrimSpace(event.Get("name").ToString())
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
		argName := strings.TrimSpace(args.Get(i).Get("name").ToString())
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
		respName := strings.TrimSpace(response.Get(i).Get("name").ToString())
		if _, seen := respNameSet[respName]; seen {
			return fmt.Errorf("response[%d]: repeat resp name: %q", i, respName)
		} else {
			respNameSet[respName] = struct{}{}
		}
	}

	// 确保事件名不能重复
	methodName := strings.TrimSpace(method.Get("name").ToString())
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
	if strings.TrimSpace(name.ToString()) == "" {
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
	if strings.TrimSpace(description.ToString()) == "" {
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
	typeStr := strings.TrimSpace(Type.ToString())
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
			if strings.TrimSpace(unit.ToString()) == "" {
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
		valueStr := strings.TrimSpace(optionValue.ToString())
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
		if strings.TrimSpace(description.ToString()) == "" {
			return fmt.Errorf("range: option[%d]: description is empty", i)
		}
	}

	// 如果有default字段，检查默认值是否合理
	Default := rangeObj.Get("default")
	if Default.LastError() == nil {
		if Default.ValueType() != jsoniter.StringValue {
			return fmt.Errorf("range: default: NOT string")
		}

		defaultVal := strings.TrimSpace(Default.ToString())

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
			if strings.TrimSpace(description.ToString()) == "" {
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
				return fmt.Errorf("range: default: %d NOT in option", defaultVal)
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
			if strings.TrimSpace(description.ToString()) == "" {
				return fmt.Errorf("range: option[%d]: description is empty", i)
			}
		}

		// 如果有default字段，检查默认值是否合理
		Default := rangeObj.Get("default")
		if Default.LastError() == nil {
			// 默认值必须是uint
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
		tokens[i] = strings.TrimSpace(token)
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
			templateName := strings.TrimSpace(token[1 : len(token)-1])

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

const empty = `
{
	"name": "__empty__/{uuid}",
	"description": "empty model meta information",
	"state": [],
	"event": [],
	"method": []
}
`

// NewEmptyMeta 返回一个不包含任何状态、事件和方法的空元信息.
//
// NewEmptyMeta 返回的元信息可以用下面JSON串表示:
// {
//		"name": "__empty__/{uuid}",
//		"description": "empty model meta information",
//		"state": [],
//		"event": [],
//		"method": []
// }
// 其中模板参数uuid随机生成
func NewEmptyMeta() *Meta {
	ans, err := Parse([]byte(empty), TemplateParam{
		"uuid": uuid.New().String(),
	})

	if err != nil {
		panic(err)
	}

	return ans
}
