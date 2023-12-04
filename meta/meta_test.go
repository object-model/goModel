package meta

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"testing"
)

const metaJson = `
{
	"name": "test",
	"description": "测试元信息",
	"state": [
		{
			"name": "metaInfo",
			"description": "元信息",
			"type": "meta"
		}
	],
	"event": [
	],
	"method": [
	]
}
`

func newString(str string) *string {
	return &str
}

func newUint(n uint) *uint {
	return &n
}

func TestParseError(t *testing.T) {
	testCases := []struct {
		data    string
		wantErr string
		desc    string
	}{
		{
			`{"name":}`,
			"parse JSON failed",
			"解析JSON串错误1",
		},

		{
			`{`,
			"parse JSON failed",
			"解析JSON串错误2",
		},

		{
			`{}{`,
			"parse JSON failed",
			"解析JSON串错误3",
		},

		{
			`[]`,
			"root: NOT an object",
			"根节点不是对象",
		},

		{
			`{"description": "123"}`,
			"root: name NOT exist",
			"name字段不存在",
		},

		{
			`{"name": false}`,
			"root: name is NOT string",
			"name字段类型不正确",
		},

		{
			`{"name": "  "}`,
			"root: name is empty",
			"name字段为空字符串",
		},

		{
			`{"name": "test"}`,
			"root: description NOT exist",
			"description字段不存在",
		},

		{
			`{"name": "test", "description": {}}`,
			"root: description is NOT string",
			"description字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型"}`,
			"root: state NOT exist",
			"state字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": {}}`,
			"root: state is NOT array",
			"state字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": []}`,
			"root: event NOT exist",
			"event字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": 123}`,
			"root: event is NOT array",
			"event字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": []}`,
			"root: method NOT exist",
			"method字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": false}`,
			"root: method is NOT array",
			"method字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [123], "event": [], "method": []}`,
			"state[0]: NOT object",
			"状态不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{}], "event": [], "method": []}`,
			"state[0]: name NOT exist",
			"状态对象缺少name",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": 123}], "event": [], "method": []}`,
			"state[0]: name is NOT string",
			"状态对象name字段不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1"}], "event": [], "method": []}`,
			"state[0]: description NOT exist",
			"状态对象缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": 3.14}], "event": [], "method": []}`,
			"state[0]: description is NOT string",
			"状态对象description字段不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "   "}], "event": [], "method": []}`,
			"state[0]: description is empty",
			"状态对象description为空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1"}], "event": [], "method": []}`,
			"state[0]: type NOT exist",
			"状态对象缺少type",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": []}], "event": [], "method": []}`,
			"state[0]: type is NOT string",
			"状态对象type字段不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "    "}], "event": [], "method": []}`,
			"state[0]: type is empty",
			"状态对象type字段是空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "map"}], "event": [], "method": []}`,
			"state[0]: invalid type: \"map\"",
			"状态对象type字段为无效类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "array"}], "event": [], "method": []}`,
			"state[0]: length NOT exist in array",
			"array类型的状态缺少length字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "array", "length": "3.1"}], "event": [], "method": []}`,
			"state[0]: length is NOT number",
			"array类型的状态的length字段不是数值",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "array", "length": 3.1}], "event": [], "method": []}`,
			"state[0]: length is NOT uint",
			"array类型的状态的length字段不是uint",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "array", "length": 0}], "event": [], "method": []}`,
			"state[0]: length is NOT greater than 0",
			"array类型的状态的length小于等于0",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "array", "length":5}], "event": [], "method": []}`,
			"state[0]: element NOT exist",
			"array类型的状态没有element字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "state1", "description": "状态1", "type": "slice"}], "event": [], "method": []}`,
			"state[0]: element NOT exist",
			"slice类型的状态没有element字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "array", "length":5, "element": []}], "event": [], "method": []}`,
			"state[0]: element: NOT object",
			"array类型的状态的element字段不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "array", "length":5, "element": {}}], "event": [], "method": []}`,
			"state[0]: element: type NOT exist",
			"array类型的状态的element字段缺少type字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "array", "length":5, "element": {"type": 123}}], "event": [], "method": []}`,
			"state[0]: element: type is NOT string",
			"array类型的状态的element字段的type字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "array", "length":5, "element": {"type": "Set"}}], "event": [], "method": []}`,
			"state[0]: element: invalid type: \"Set\"",
			"array类型的状态的element字段的type字段表示类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "sensor", "description": "状态1", "type": "struct"}], "event": [], "method": []}`,
			"state[0]: fields NOT exist",
			"struct类型的状态的fields字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "sensor", "description": "状态1", "type": "struct", "fields": {}}], "event": [], "method": []}`,
			"state[0]: fields is NOT array",
			"struct类型的状态的fields字段不为数组类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "sensor", "description": "状态1", "type": "struct", "fields": [{}]}], "event": [], "method": []}`,
			"state[0]: fields[0]: name NOT exist",
			"struct类型的状态的fields字段中的项目缺少name字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "sensor", "description": "状态1", "type": "struct", "fields": [{ "name": "a", "description": "子字段a", "type": "Str" }]}], "event": [], "method": []}`,
			"state[0]: fields[0]: invalid type: \"Str\"",
			"struct类型的状态的fields字段中的项目的type类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "sensor", "description": "状态1", "type": "struct", "fields": [{ "name": "a", "description": "子字段a", "type": "float"}, { "name": "a", "description": "子字段a", "type": "float"}]}], "event": [], "method": []}`,
			"state[0]: fields[1]: repeat field name: \"a\"",
			"struct类型的状态的存在重复字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "slice", "element": {"type": 123}}], "event": [], "method": []}`,
			"state[0]: element: type is NOT string",
			"slice类型的状态的element字段的type字段类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "unit": 123}], "event": [], "method": []}`,
			"state[0]: unit is NOT string",
			"unit不是string类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "unit": "   "}], "event": [], "method": []}`,
			"state[0]: unit is empty",
			"unit是空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": 123}], "event": [], "method": []}`,
			"state[0]: range: NOT object",
			"range类型不正确",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "nums", "description": "状态1", "type": "slice", "element": {"type": "float"}, "range": {}}], "event": [], "method": []}`,
			"state[0]: range: \"slice\" NOT support range",
			"在不支持的类型上使用range",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {}}], "event": [], "method": []}`,
			"state[0]: range: NO min or max for float range",
			"float类型中的range中的既没有min也没有max",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": "haha"}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT number",
			"float类型中的range中的min类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": -10, "max": "nan"}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT number",
			"float类型中的range中的max类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": 10, "max": -10}}], "event": [], "method": []}`,
			"state[0]: range: min is NOT less than max",
			"float类型中的range中的max小于min",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": -10, "max": 9.9, "default": "nan"}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT number",
			"float类型中的range中的default类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": -10, "max": 9.9, "default": -10.1}}], "event": [], "method": []}`,
			"state[0]: range: default: less than min",
			"float类型中的range中的default小于最低值",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "vol", "description": "状态1", "type": "float", "range": {"min": -10, "max": 9.9, "default": 9.9001}}], "event": [], "method": []}`,
			"state[0]: range: default: greater than max",
			"float类型中的range中的default大于最大值",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {}}], "event": [], "method": []}`,
			"state[0]: range: NO option for string range",
			"string类型中的range中没有option",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": {}}}], "event": [], "method": []}`,
			"state[0]: range: option: NOT array",
			"string类型中的range中的option类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": []}}], "event": [], "method": []}`,
			"state[0]: range: option: size less than 1",
			"string类型中的range中的option包含0个选项",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [123, 123]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: NOT object",
			"string类型中的range中的option的选项不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value NOT exist",
			"string类型中的range中的option的选项缺少value",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": true}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT string",
			"string类型中的range中的option的选项中的value不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "  "}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is empty",
			"string类型中的range中的option的选项中的value为空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast"}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description NOT exist",
			"string类型中的range中的option的选项缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": 123}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is NOT string",
			"string类型中的range中的option的选项中的description不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "   "}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is empty",
			"string类型中的range中的option的选项中的description为空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "快"}, {}]}}], "event": [], "method": []}`,
			"state[0]: range: option[1]: value NOT exist",
			"string类型中的range中的option[1]的选项缺少value",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "快"}, {"value": "fast", "description": "慢"}]}}], "event": [], "method": []}`,
			"state[0]: range: option[1]: repeat value: \"fast\"",
			"string类型中的range中的option[1]的选项重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "快"}, {"value": "middle", "description": "中"}], "default": 123}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT string",
			"string类型中的range中的default不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "快"}, {"value": "middle", "description": "中"}], "default": "  "}}], "event": [], "method": []}`,
			"state[0]: range: default is empty",
			"string类型中的range中的default为空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "speed", "description": "状态1", "type": "string", "range": {"option": [{"value": "fast", "description": "快"}, {"value": "middle", "description": "中"}], "default": "slow"}}], "event": [], "method": []}`,
			"state[0]: range: default: \"slow\" NOT in option",
			"string类型中的range中的default值不在选项内",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"min": "haha"}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT number",
			"int类型中的range中的min类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"min": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT int",
			"int类型中的range中的min不是int",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": "haha"}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT number",
			"int类型中的range中的max类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT int",
			"int类型中的range中的max不是int",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {}}], "event": [], "method": []}`,
			"state[0]: range: NO min and max for int range",
			"int类型中的range中的max和min都不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": -10, "min": 10}}], "event": [], "method": []}`,
			"state[0]: range: min is NOT less than max",
			"int类型中的range中的max小于min",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": 10, "min": -10, "default": "3"}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT number",
			"int类型中的range中的default类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": 10, "min": -10, "default": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT int",
			"int类型中的range中的default不是int",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"min": -10, "default": -11}}], "event": [], "method": []}`,
			"state[0]: range: default: less than min",
			"int类型中的range中的default小于min",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"max": 10, "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: default: greater than max",
			"int类型中的range中的default大于max",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": {}, "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option: NOT array",
			"int类型中的range中的option不是数组",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [], "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option: size less than 1",
			"int类型中的range中的option包含0个选项",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [3], "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: NOT object",
			"int类型中的range中的option[0]不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{}], "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value NOT exist",
			"int类型中的range中的option[0]缺少value字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": "31.4"}], "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT number",
			"int类型中的range中的option[0]的value不是数值类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 31.4}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT int",
			"int类型中的range中的option[0]的value不是int",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description NOT exist",
			"int类型中的range中的option[0]缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": 3.14}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is NOT string",
			"int类型中的range中的option[0]的description不是string",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": ""}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is empty",
			"int类型中的range中的option[0]的description为空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 1, "description": "选项2"} ]}}], "event": [], "method": []}`,
			"state[0]: range: option[1]: repeat value: 1",
			"int类型中的range中的option[1]的选项重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"}], "default": "123"}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT number",
			"int类型中的range中的default类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"}], "default": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT int",
			"int类型中的range中的default类型错误,不是int",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "int", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"}], "default": 3}}], "event": [], "method": []}`,
			"state[0]: range: default: 3 NOT in option",
			"int类型中的range中的default不再可选项中",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {}}], "event": [], "method": []}`,
			"state[0]: range: NO min or max for uint range",
			"uint类型中的range中既没有min也没有max",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"min": "haha"}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT number",
			"uint类型中的range中的min类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"min": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT uint",
			"uint类型中的range中的min不是uint",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"min": -1}}], "event": [], "method": []}`,
			"state[0]: range: min: NOT uint",
			"uint类型中的range中的min不是uint, 是负数",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": "haha"}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT number",
			"uint类型中的range中的max类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT uint",
			"uint类型中的range中的max不是uint",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": -2}}], "event": [], "method": []}`,
			"state[0]: range: max: NOT uint",
			"uint类型中的range中的min不是uint, 是负数",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 1, "min": 10}}], "event": [], "method": []}`,
			"state[0]: range: min is NOT less than max",
			"uint类型中的range中的min大于max",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 10, "min": 1, "default": "3"}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT number",
			"uint类型中的range中的default类型错误",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 10, "min": 1, "default": 3.14}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT uint",
			"uint类型中的range中的default不是uint",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 10, "min": 1, "default": -1}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT uint",
			"uint类型中的range中的default不是uint, 是负数",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 10, "min": 1, "default": 0}}], "event": [], "method": []}`,
			"state[0]: range: default: less than min",
			"uint类型中的range中的default小于min",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"max": 10, "min": 1, "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: default: greater than max",
			"uint类型中的range中的default大于max",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": {}}}], "event": [], "method": []}`,
			"state[0]: range: option: NOT array",
			"uint类型中的range中的option不是数组",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": []}}], "event": [], "method": []}`,
			"state[0]: range: option: size less than 1",
			"uint类型中的range中的option包含0个选项",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [123]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: NOT object",
			"uint类型中的range中的option[0]不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value NOT exist",
			"uint类型中的range中的option[0]缺少value字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": "31.4"}], "default": 11}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT number",
			"uint类型中的range中的option[0]的value不是数值类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 31.4}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT uint",
			"uint类型中的range中的option[0]的value不是uint",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": -1}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: value is NOT uint",
			"uint类型中的range中的option[0]的value不是uint, 是负数",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description NOT exist",
			"uint类型中的range中的option[0]缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": 3.14}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is NOT string",
			"uint类型中的range中的option[0]的description不是string",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": "  "}]}}], "event": [], "method": []}`,
			"state[0]: range: option[0]: description is empty",
			"uint类型中的range中的option[0]的description是空字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 1, "description": "选项2"} ]}}], "event": [], "method": []}`,
			"state[0]: range: option[1]: repeat value: 1",
			"uint类型中的range中的option[1]的选项重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"} ], "default": "123"}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT number",
			"uint类型中的range中的default不是数值类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"} ], "default": -1}}], "event": [], "method": []}`,
			"state[0]: range: default: NOT uint",
			"uint类型中的range中的default不是uint类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint", "range": {"option": [{"value": 1, "description": "选项1"}, {"value": 2, "description": "选项2"} ], "default": 0}}], "event": [], "method": []}`,
			"state[0]: range: default: 0 NOT in option",
			"uint类型中的range中的default的值不在可选项中",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [{"name": "temp", "description": "状态1", "type": "uint"}, {"name": "temp", "description": "状态1", "type": "uint"}], "event": [], "method": []}`,
			"state[1]: repeat state name: \"temp\"",
			"重复的状态名",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [2.1], "method": []}`,
			"event[0]: NOT object",
			"事件元信息不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{}], "method": []}`,
			"event[0]: name NOT exist",
			"事件元信息缺少name字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": 123}], "method": []}`,
			"event[0]: name is NOT string",
			"事件元信息的name字段不是字符串类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok"}], "method": []}`,
			"event[0]: description NOT exist",
			"事件元信息缺少description字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": 1}], "method": []}`,
			"event[0]: description is NOT string",
			"事件元信息的description字段不是字符串类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成"}], "method": []}`,
			"event[0]: NO args",
			"事件元信息的args字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成", "args": 123}], "method": []}`,
			"event[0]: args is NOT array",
			"事件元信息的args字段不是数组类型",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成", "args": [{"name": "time", "description": "时间"}]}], "method": []}`,
			"event[0]: args[0]: type NOT exist",
			"事件元信息event[0]的args[0]缺少type",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成", "args": [{"name": "time"}]}], "method": []}`,
			"event[0]: args[0]: description NOT exist",
			"事件元信息event[0]的args[0]缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成", "args": [{"name": "time", "description": "时间", "type": "Uint32"}]}], "method": []}`,
			"event[0]: args[0]: invalid type: \"Uint32\"",
			"事件元信息event[0]的args[0]的type出错",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "ok", "description": "完成", "args": [{"name": "  time", "description": "时间", "type": "uint"}, {"name": "time  ", "description": "时间", "type": "uint"}]}], "method": []}`,
			"event[0]: args[1]: repeat arg name: \"time\"",
			"事件元信息event[0]的args[1]的名称重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [{"name": "  ok", "description": "完成", "args": [{"name": "time", "description": "时间", "type": "uint"}, {"name": "msg", "description": "提示信息", "type": "string"}]}, {"name": "ok  ", "description": "完成", "args": [{"name": "time", "description": "时间", "type": "uint"}, {"name": "msg", "description": "提示信息", "type": "string"}]}], "method": []}`,
			"event[1]: repeat event name: \"ok\"",
			"事件元信息event[1]的事件名重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [123]}`,
			"method[0]: NOT object",
			"方法元信息不是对象",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{}]}`,
			"method[0]: name NOT exist",
			"方法元信息缺少name",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": 123}]}`,
			"method[0]: name is NOT string",
			"方法元信息的name不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS"}]}`,
			"method[0]: description NOT exist",
			"方法元信息缺少description",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": 123}]}`,
			"method[0]: description is NOT string",
			"方法元信息的description不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖"}]}`,
			"method[0]: NO args",
			"方法元信息缺少args",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": {}}]}`,
			"method[0]: args is NOT array",
			"方法元信息的args不是数组",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{}]}]}`,
			"method[0]: args[0]: name NOT exist",
			"方法元信息的args[0]的name不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": 123}]}]}`,
			"method[0]: args[0]: name is NOT string",
			"方法元信息的args[0]的name不是字符串",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed"}]}]}`,
			"method[0]: args[0]: description NOT exist",
			"方法元信息的args[0]的description不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度"}]}]}`,
			"method[0]: args[0]: type NOT exist",
			"方法元信息的args[0]的type不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}, {"name": "speed", "description": "速度", "type": "string"}]}]}`,
			"method[0]: args[1]: repeat arg name: \"speed\"",
			"方法元信息的args[1]的名称重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}]}]}`,
			"method[0]: NO response",
			"方法元信息的response字段不存在",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}], "response": 123}]}`,
			"method[0]: response is NOT array",
			"方法元信息的response字段不是数组",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}], "response": [{"name": "res", "description": "结果"}]}]}`,
			"method[0]: response[0]: type NOT exist",
			"方法元信息的response[0]缺少type字段",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": "QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}], "response": [{"name": "res", "description": "结果", "type": "bool"}, {"name": "  res   ", "description": "结果", "type": "bool"}]}]}`,
			"method[0]: response[1]: repeat resp name: \"res\"",
			"方法元信息的response[1]的名称重复",
		},

		{
			`{"name": "test", "description": "测试物模型", "state": [], "event": [], "method": [{"name": " QS", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}], "response": [{"name": "res", "description": "结果", "type": "bool"}]}, {"name": "QS   ", "description": "起竖", "args": [{"name": "speed", "description": "速度", "type": "string"}], "response": [{"name": "res", "description": "结果", "type": "bool"}]}]}`,
			"method[1]: repeat method name: \"QS\"",
			"方法名称重复",
		},

		{
			`{"name": " / ", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			"root: name: empty model name after normalize",
			"空的物模型名称",
		},

		{
			`{"name": " / {   }", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			"root: name: empty template name",
			"空的模板名称",
		},

		{
			`{"name": "car / {  { a }", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			"root: name: template \"{ a\": contains extra '{'",
			"模板名包含多余的{",
		},

		{
			`{"name": "car / {  a} }", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			"root: name: template \"a}\": contains extra '}'",
			"模板名包含多余的}",
		},

		{
			`{"name": "car / {a} / {  a  }", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			"root: name: repeat template: \"a\"",
			"重复的模板名称",
		},
	}

	for _, test := range testCases {
		_, err := Parse([]byte(test.data), nil)
		assert.NotNil(t, err)
		assert.Equal(t, test.wantErr, err.Error(), test.desc)
	}
}

func TestParseWithTemplateError(t *testing.T) {
	testCases := []struct {
		data    string
		param   TemplateParam
		wantErr string
		desc    string
	}{
		{
			`{"name": "{group/car / {id}/ tpqs", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			TemplateParam{
				" id ": " #1 ",
			},
			"root: name: template \"{group\": missing '}'",
			"模板格式出错1",
		},

		{
			`{"name": "group}/car / {id}/ tpqs", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			TemplateParam{
				" id ": " #1 ",
			},
			"root: name: template \"group}\": missing '{'",
			"模板格式出错2",
		},

		{
			`{"name": "{group}/car / {id}/ tpqs", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			TemplateParam{
				" id ": " #1 ",
			},
			"template \"group\": missing",
			"模板参数缺失",
		},

		{
			`{"name": "{group}/car / {id}/ tpqs", "description": "测试物模型", "state": [], "event": [], "method": []}`,
			TemplateParam{
				"group": " A ",
				" id ":  "  ",
			},
			"template \"id\": value is empty",
			"模板参为空",
		},
	}

	for _, test := range testCases {
		_, err := Parse([]byte(test.data), test.param)
		assert.NotNil(t, err)
		assert.Equal(t, test.wantErr, err.Error(), test.desc)
	}
}

func TestParseOk(t *testing.T) {

	meta := Meta{
		Name:        "A/car/#1/tpqs",
		Description: "发射车调平起竖服务",
		State: []ParamMeta{
			{
				Name:        newString("tpqsInfo"),
				Description: newString("调平起竖状态信息"),
				Type:        "struct",
				Fields: []ParamMeta{
					{
						Name:        newString("qsState"),
						Description: newString("起竖状态"),
						Type:        "string",
						Range: &RangeInfo{
							Option: []OptionInfo{
								{
									Value:       "erecting",
									Description: "起竖中",
								},

								{
									Value:       "hping",
									Description: "回平中",
								},

								{
									Value:       "uping",
									Description: "升车中",
								},

								{
									Value:       "downing",
									Description: "将车中",
								},
							},
						},
					},

					{
						Name:        newString("hpSwitch"),
						Description: newString("回平到位开关是否有效"),
						Type:        "bool",
					},

					{
						Name:        newString("qsAngle"),
						Description: newString("起竖角度"),
						Type:        "float",
						Range: &RangeInfo{
							Max: float64(200),
							Min: float64(0),
						},
						Unit: newString("度"),
					},

					{
						Name:        newString("errors"),
						Description: newString("起竖系统故障信息"),
						Type:        "slice",
						Element: &ParamMeta{
							Type: "struct",
							Fields: []ParamMeta{
								{
									Name:        newString("code"),
									Description: newString("故障码值"),
									Type:        "uint",
									Range: &RangeInfo{
										Max: uint(1000),
										Min: uint(1),
									},
								},

								{
									Name:        newString("msg"),
									Description: newString("故障提示信息"),
									Type:        "string",
								},
							},
						},
					},
				},
			},

			{
				Name:        newString("powerInfo"),
				Description: newString("8路配电通道信息"),
				Type:        "array",
				Element: &ParamMeta{
					Type: "struct",
					Fields: []ParamMeta{
						{
							Name:        newString("isOn"),
							Description: newString("配电是否接通"),
							Type:        "bool",
						},

						{
							Name:        newString("outCur"),
							Description: newString("配电输出电流"),
							Type:        "float",
							Range: &RangeInfo{
								Max: float64(100000),
								Min: float64(-100000),
							},
							Unit: newString("A"),
						},
					},
				},
				Length: newUint(8),
			},
		},
		Event: []EventMeta{
			{
				Name:        "qsMotorOverCur",
				Description: "起竖电机过流告警事件",
				Args:        make([]ParamMeta, 0),
			},

			{
				Name:        "qsAction",
				Description: "起竖动作中事件",
				Args: []ParamMeta{
					{
						Name:        newString("motors"),
						Description: newString("4路起竖电机状态"),
						Type:        "array",
						Element: &ParamMeta{
							Type: "struct",
							Fields: []ParamMeta{
								{
									Name:        newString("rov"),
									Description: newString("电机转速"),
									Type:        "int",
									Unit:        newString("rpm"),
								},

								{
									Name:        newString("cur"),
									Description: newString("电机电流"),
									Type:        "int",
									Unit:        newString("mA"),
								},

								{
									Name:        newString("temp"),
									Description: newString("电机温度"),
									Type:        "int",
									Unit:        newString("℃"),
								},
							},
						},
						Length: newUint(4),
					},

					{
						Name:        newString("qsAngle"),
						Description: newString("起竖角度"),
						Type:        "float",
						Unit:        newString("°"),
					},
				},
			},
		},
		Method: []MethodMeta{
			{
				Name:        "QS",
				Description: "起竖控制",
				Args: []ParamMeta{
					{
						Name:        newString("angle"),
						Description: newString("期望的起竖角度"),
						Type:        "float",
						Range: &RangeInfo{
							Max:     float64(91),
							Min:     float64(0),
							Default: float64(90),
						},
						Unit: newString("°"),
					},

					{
						Name:        newString("speed"),
						Description: newString("起竖速度选择"),
						Type:        "string",
						Range: &RangeInfo{
							Option: []OptionInfo{
								{
									Value:       "slow",
									Description: "慢速",
								},

								{
									Value:       "middle",
									Description: "中速",
								},

								{
									Value:       "fast",
									Description: "快速",
								},

								{
									Value:       "superFast",
									Description: "特快速",
								},
							},
							Default: "superFast",
						},
					},
				},
				Response: []ParamMeta{
					{
						Name:        newString("res"),
						Description: newString("执行是否成功"),
						Type:        "bool",
					},

					{
						Name:        newString("msg"),
						Description: newString("执行结果的描述信息,执行失败时描述失败原因"),
						Type:        "string",
					},

					{
						Name:        newString("time"),
						Description: newString("执行时间"),
						Type:        "uint",
						Range: &RangeInfo{
							Max: uint(100000),
							Min: uint(0),
						},
						Unit: newString("ms"),
					},

					{
						Name:        newString("code"),
						Description: newString("执行结果码"),
						Type:        "int",
						Range: &RangeInfo{
							Option: []OptionInfo{
								{
									Value:       0,
									Description: "执行成功",
								},

								{
									Value:       1,
									Description: "起竖传感器离线",
								},

								{
									Value:       2,
									Description: "驱动器未上电",
								},

								{
									Value:       3,
									Description: "未处于开盖状态",
								},
							},
						},
					},
				},
			},
		},

		nameTokens: []string{
			"A",
			"car",
			"#1",
			"tpqs",
		},

		nameTemplates: map[string]int{
			"group": 0,
			"id":    2,
		},

		stateIndex: map[string]int{
			"tpqsInfo":  0,
			"powerInfo": 1,
		},

		eventIndex: map[string]int{
			"qsMotorOverCur": 0,
			"qsAction":       1,
		},

		methodIndex: map[string]int{
			"QS": 0,
		},
	}

	json, _ := ioutil.ReadFile("./tpqs.json")
	m, err := Parse(json, TemplateParam{
		" group": "  A  ",
		" id  ":  " #1",
	})
	assert.Nil(t, err)
	assert.EqualValues(t, meta, m)

	assert.EqualValues(t, []string{
		"A/car/#1/tpqs/tpqsInfo",
		"A/car/#1/tpqs/powerInfo",
	}, m.AllStates())

	assert.EqualValues(t, []string{
		"A/car/#1/tpqs/qsMotorOverCur",
		"A/car/#1/tpqs/qsAction",
	}, m.AllEvents())

	assert.EqualValues(t, []string{
		"A/car/#1/tpqs/QS",
	}, m.AllMethods())
}

func TestMeta_VerifyStateError(t *testing.T) {
	json, _ := ioutil.ReadFile("./tpqs.json")
	m, err := Parse(json, TemplateParam{
		" group": "  A  ",
		" id  ":  " #1",
	})
	assert.Nil(t, err)

	testCases := []struct {
		name   string
		data   interface{}
		errStr string
		desc   string
	}{
		{
			name:   "unknown",
			data:   123,
			errStr: "NO state \"unknown\"",
			desc:   "不存在的状态",
		},

		{
			name:   "tpqsInfo",
			data:   123,
			errStr: "type unmatched",
			desc:   "状态类型不匹配",
		},

		{
			name: "tpqsInfo",
			data: struct {
			}{},
			errStr: "field \"qsState\": missing",
			desc:   "缺失字段",
		},

		{
			name: "tpqsInfo",
			data: struct {
				qsState string `json:"qsState"`
			}{},
			errStr: "field \"qsState\": unexported",
			desc:   "字段未导出",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState float64 `json:"qsState"`
			}{},
			errStr: "field \"qsState\": type unmatched",
			desc:   "字段类型不匹配",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState string `json:"qsState"`
			}{
				QSState: "unknown",
			},
			errStr: "field \"qsState\": \"unknown\" NOT in option",
			desc:   "字段值不在范围的可选项中",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch int8    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
			}{
				QSState:  "erecting",
				HPSwitch: 0,
				QSAngle:  -1.2,
			},
			errStr: "field \"hpSwitch\": type unmatched",
			desc:   "bool类型的字段类型不匹配",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch bool    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
				Errors   []struct {
					Code int    `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QSState:  "erecting",
				HPSwitch: false,
				QSAngle:  45.0,
				Errors: []struct {
					Code int    `json:"code"`
					Msg  string `json:"msg"`
				}{},
			},
			errStr: "field \"errors\": element: field \"code\": type unmatched",
			desc:   "切片类型的子字段值为空，但切片元素的类型不正确",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch bool    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
				Errors   []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QSState:  "erecting",
				HPSwitch: false,
				QSAngle:  45.0,
			},
			errStr: "field \"errors\": nil slice",
			desc:   "切片元素的类型正确，但为nil切片",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch bool    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
				Errors   []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QSState:  "erecting",
				HPSwitch: false,
				QSAngle:  45.0,
				Errors: []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				}{
					{
						Code: 1001,
						Msg:  "位置消息",
					},
				},
			},
			errStr: "field \"errors\": element[0]: field \"code\": greater than max",
			desc:   "切片中每个元素超限",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch bool    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
			}{
				QSState:  "erecting",
				HPSwitch: false,
				QSAngle:  -1.2,
			},
			errStr: "field \"qsAngle\": less than min",
			desc:   "字段值小于最小值",
		},

		{
			name: "tpqsInfo",
			data: struct {
				QSState  string  `json:"qsState"`
				HPSwitch bool    `json:"hpSwitch"`
				QSAngle  float64 `json:"qsAngle"`
			}{
				QSState:  "erecting",
				HPSwitch: false,
				QSAngle:  200.1,
			},
			errStr: "field \"qsAngle\": greater than max",
			desc:   "字段值大于最大值",
		},

		{
			name: "powerInfo",
			data: [4]struct {
				IsOn   bool    `json:"isOn"`
				OutCur float64 `json:"outCur"`
			}{},
			errStr: "length NOT equal to 8",
			desc:   "数组类型状态长度错误",
		},

		{
			name: "powerInfo",
			data: [8]struct {
				IsOn   int8    `json:"isOn"`
				OutCur float64 `json:"outCur"`
			}{},
			errStr: "element: field \"isOn\": type unmatched",
			desc:   "数组元素类型不匹配",
		},

		{
			name: "powerInfo",
			data: [8]struct {
				IsOn   bool    `json:"isOn"`
				OutCur float32 `json:"outCur"`
			}{
				{
					IsOn:   true,
					OutCur: 100000,
				},

				{
					IsOn:   true,
					OutCur: 100001,
				},
			},
			errStr: "element[1]: field \"outCur\": greater than max",
			desc:   "数组中某一项的某个字段超限",
		},
	}

	for _, test := range testCases {
		err = m.VerifyState(test.name, test.data)
		assert.NotNil(t, err, test.desc)
		assert.EqualValues(t, test.errStr, err.Error(), test.desc)
	}
}

func TestMeta_VerifyStateMetaError(t *testing.T) {
	m, err := Parse([]byte(metaJson), nil)
	assert.Nil(t, err)

	type TestCase struct {
		MetaData Meta
		ErrStr   string
		Desc     string
	}

	testCases := []TestCase{
		{
			MetaData: Meta{},
			ErrStr:   "root: name is empty",
			Desc:     "元信息name为空",
		},

		{
			MetaData: Meta{
				Name: "meta-info",
			},
			ErrStr: "root: description is empty",
			Desc:   "元信息name为空",
		},

		{
			MetaData: Meta{
				Name:        "测试元信息",
				Description: "测试元信息",
			},
			ErrStr: "root: state is NOT array",
			Desc:   "元信息state为空",
		},

		{
			MetaData: Meta{
				Name:        "测试元信息",
				Description: "测试元信息",
				State:       make([]ParamMeta, 0),
			},
			ErrStr: "root: event is NOT array",
			Desc:   "元信息event为空",
		},

		{
			MetaData: Meta{
				Name:        "测试元信息",
				Description: "测试元信息",
				State:       make([]ParamMeta, 0),
				Event:       make([]EventMeta, 0),
			},
			ErrStr: "root: method is NOT array",
			Desc:   "元信息method为空",
		},

		{
			MetaData: Meta{
				Name:        "测试元信息",
				Description: "测试元信息",
				State: []ParamMeta{
					{
						Type: "int",
					},
				},
				Event:  make([]EventMeta, 0),
				Method: make([]MethodMeta, 0),
			},
			ErrStr: "state[0]: name NOT exist",
			Desc:   "元信息的状态名称不存在",
		},

		{
			MetaData: Meta{
				Name:        "测试元信息",
				Description: "测试元信息",
				State: []ParamMeta{
					{
						Name: newString("state1"),
						Type: "int",
					},
				},
				Event:  make([]EventMeta, 0),
				Method: make([]MethodMeta, 0),
			},
			ErrStr: "state[0]: description NOT exist",
			Desc:   "元信息的状态描述不存在",
		},
	}

	for _, test := range testCases {
		err := m.VerifyState("metaInfo", test.MetaData)
		assert.NotNil(t, err, test.Desc)
		assert.EqualValues(t, test.ErrStr, err.Error())
	}

}

func TestMeta_VerifyStateOK(t *testing.T) {
	json, _ := ioutil.ReadFile("./tpqs.json")
	m, err := Parse(json, TemplateParam{
		" group": "  A  ",
		" id  ":  " #1",
	})
	assert.Nil(t, err)

	type TestCase struct {
		Name string
		Data interface{}
		Desc string
	}

	testCases := []TestCase{
		{
			Name: "tpqsInfo",
			Data: struct {
				QsState  string  `json:"qsState"`
				HpSwitch bool    `json:"hpSwitch"`
				QsAngle  float32 `json:"qsAngle"`
				Errors   [2]struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QsState:  "hping",
				HpSwitch: false,
				QsAngle:  89.6,
				Errors: [2]struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				}{
					{
						Code: 1,
						Msg:  "温度超限",
					},

					{
						Code: 2,
						Msg:  "电压超限",
					},
				},
			},
			Desc: "正常状态1",
		},

		{
			Name: "tpqsInfo",
			Data: struct {
				QsState  string  `json:"qsState"`
				HpSwitch bool    `json:"hpSwitch"`
				QsAngle  float64 `json:"qsAngle"`
				Errors   []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QsState:  "hping",
				HpSwitch: false,
				QsAngle:  0.0,
				Errors: []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				}{
					{
						Code: 1,
						Msg:  "温度超限",
					},

					{
						Code: 2,
						Msg:  "电压超限",
					},
				},
			},
			Desc: "正常状态2",
		},

		{
			Name: "tpqsInfo",
			Data: struct {
				QsState  string  `json:"qsState"`
				HpSwitch bool    `json:"hpSwitch"`
				QsAngle  float64 `json:"qsAngle"`
				Errors   []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				} `json:"errors"`
			}{
				QsState:  "downing",
				HpSwitch: false,
				QsAngle:  200.0,
				Errors: []struct {
					Code uint   `json:"code"`
					Msg  string `json:"msg"`
				}{},
			},
			Desc: "正常状态3",
		},
	}

	for _, test := range testCases {
		err := m.VerifyState(test.Name, test.Data)
		assert.Nil(t, err, test.Desc)
	}
}

func TestMeta_VerifyEventError(t *testing.T) {
	json, _ := ioutil.ReadFile("./tpqs.json")
	m, err := Parse(json, TemplateParam{
		" group": "  A  ",
		" id  ":  " #1",
	})
	assert.Nil(t, err)

	testCases := []struct {
		name   string
		args   interface{}
		errStr string
		desc   string
	}{
		{
			name:   "unknown",
			args:   "null",
			errStr: "NO event \"unknown\"",
			desc:   "不存在的事件",
		},

		{
			name:   "qsMotorOverCur",
			args:   "hello",
			errStr: "args: NOT an struct",
			desc:   "事件参数类型不匹配",
		},

		{
			name: "qsAction",
			args: struct {
			}{},
			errStr: "arg \"motors\": missing",
			desc:   "事件参数缺失",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]float32
			}{},
			errStr: "arg \"motors\": missing",
			desc:   "事件参数缺失--没有json标签",
		},

		{
			name: "qsAction",
			args: struct {
				motors [4]float32 `json:"motors"`
			}{},
			errStr: "arg \"motors\": unexported",
			desc:   "事件参数为非导出字段",
		},

		{
			name: "qsAction",
			args: struct {
				Motors []int `json:"motors"`
			}{},
			errStr: "arg \"motors\": type unmatched",
			desc:   "事件参数类型不匹配",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [5]struct {
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": length NOT equal to 4",
			desc:   "数组类型的事件参数长度不匹配",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"rov\": missing",
			desc:   "事件参数的子字段缺失",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
					rov int `json:"rov"`
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"rov\": unexported",
			desc:   "事件参数的子字段非导出",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
					Rov uint `json:"rov"`
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"rov\": type unmatched",
			desc:   "事件参数的子字段类型不匹配-rov为uint",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
					Rov int16   `json:"rov"`
					Cur float64 `json:"cur"`
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"cur\": type unmatched",
			desc:   "事件参数的子字段类型不匹配-cur为float",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
					Rov  int16  `json:"rov"`
					Cur  int32  `json:"cur"`
					Temp string `json:"temp"`
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"temp\": type unmatched",
			desc:   "事件参数的子字段类型不匹配-temp为string",
		},

		{
			name: "qsAction",
			args: struct {
				Motors [4]struct {
					Rov  int64   `json:"rov"`
					Cur  int32   `json:"cur"`
					Temp float32 `json:"temp"`
				} `json:"motors"`
			}{},
			errStr: "arg \"motors\": element: field \"temp\": type unmatched",
			desc:   "事件参数的子字段类型不匹配-temp为float32",
		},
	}

	for _, test := range testCases {
		err = m.VerifyEvent(test.name, test.args)
		assert.NotNil(t, err, test.desc)
		assert.EqualValues(t, test.errStr, err.Error(), test.desc)
	}
}
