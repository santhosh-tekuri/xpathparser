// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xpath_test

import (
	"encoding/xml"
	"fmt"
	"testing"

	. "github.com/santhosh-tekuri/xpath"
)

func TestInvalidXPaths(t *testing.T) {
	tests := []string{
		``,
		`1.2.3`,
		`"one`,
		`'one`,
		`hero::*`,
		`$`,
		`$$`,
		`+`,
		`!`,
		`!=`,
		`abc def`,
		`abc and`,
		`child::`,
		`/abc/`,
		`abc/`,
		`@`,
		`/@`,
		`child::abcd()`,
		`;abc`,
		`abc;def`,
		`/;`,
		`[`,
		`a[`,
		`a[1`,
		`a[]`,
		`@-name`,
		`@1one`,
		`@.one`,
		`abc^def`,
		`abc#def`,
		`foo(`,
		`foo(1`,
		`foo(1,`,
		`a|`,
		`//`,
		`//+1`,
	}
	for _, test := range tests {
		if _, err := Parse(test); err == nil {
			t.Errorf("FAIL: error expected for %s", test)
		} else {
			t.Log(err)
		}
	}
}

func TestCompiledXPaths(t *testing.T) {
	tests := map[string]Expr{
		`1`:    Number(1),
		`-1`:   &NegateExpr{Number(1)},
		`1.5`:  Number(1.5),
		`.5`:   Number(.5),
		`01.5`: Number(1.5),
		`1+2`:  &BinaryExpr{Number(1), Add, Number(2)},
		`1-2`:  &BinaryExpr{Number(1), Subtract, Number(2)},
		`1*2`:  &BinaryExpr{Number(1), Multiply, Number(2)},
		`1+2*3`: &BinaryExpr{
			Number(1),
			Add,
			&BinaryExpr{Number(2), Multiply, Number(3)},
		},
		`(1+2)*3`: &BinaryExpr{
			&BinaryExpr{Number(1), Add, Number(2)},
			Multiply,
			Number(3),
		},
		`$var`:    &VarRef{"", "var"},
		`$ns:var`: &VarRef{"ns", "var"},
		`1=2`:     &BinaryExpr{Number(1), EQ, Number(2)},
		`1!=2`:    &BinaryExpr{Number(1), NEQ, Number(2)},
		`1 and 2`: &BinaryExpr{Number(1), And, Number(2)},
		`1 or2`:   &BinaryExpr{Number(1), Or, Number(2)},
		`1 mod2`:  &BinaryExpr{Number(1), Mod, Number(2)},
		`1 div2`:  &BinaryExpr{Number(1), Div, Number(2)},
		`1 <2`:    &BinaryExpr{Number(1), LT, Number(2)},
		`1 <=2`:   &BinaryExpr{Number(1), LTE, Number(2)},
		`1 >2`:    &BinaryExpr{Number(1), GT, Number(2)},
		`1 >=2`:   &BinaryExpr{Number(1), GTE, Number(2)},
		`"str"`:   String("str"),
		`'str'`:   String("str"),
		`/a`: &LocationPath{true, []*Step{
			{Child, &NameTest{"", "a"}, nil},
		}},
		`abc ander`: &BinaryExpr{
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "abc"}, nil},
			}},
			And,
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "er"}, nil},
			}},
		},
		`abc|er`: &BinaryExpr{
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "abc"}, nil},
			}},
			Union,
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "er"}, nil},
			}},
		},
		`a[1]`: &LocationPath{false, []*Step{
			{Child, &NameTest{"", "a"}, []Expr{Number(1)}},
		}},
		`foo(1)`: &FuncCall{"", "foo", []Expr{
			Number(1),
		}},
		`foo(1,2)`: &FuncCall{"", "foo", []Expr{
			Number(1),
			Number(2),
		}},
		`foo(1, ns:bar(2), /a)`: &FuncCall{"", "foo", []Expr{
			Number(1),
			&FuncCall{"ns", "bar", []Expr{
				Number(2),
			}},
			&LocationPath{true, []*Step{
				{Child, &NameTest{"", "a"}, nil},
			}},
		}},
		`.`: &LocationPath{false, []*Step{
			{Self, Node, nil},
		}},
		`..`: &LocationPath{false, []*Step{
			{Parent, Node, nil},
		}},
		`(/a/b)[5]`: &FilterExpr{
			&LocationPath{true, []*Step{
				{Child, &NameTest{"", "a"}, nil},
				{Child, &NameTest{"", "b"}, nil},
			}},
			[]Expr{Number(5)},
		},
		`(/a/b)/c`: &PathExpr{
			&LocationPath{true, []*Step{
				{Child, &NameTest{"", "a"}, nil},
				{Child, &NameTest{"", "b"}, nil},
			}},
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "c"}, nil},
			}},
		},
		`a//b`: &LocationPath{false, []*Step{
			{Child, &NameTest{"", "a"}, nil},
			{DescendantOrSelf, Node, nil},
			{Child, &NameTest{"", "b"}, nil},
		}},
		`//emp`: &LocationPath{true, []*Step{
			{DescendantOrSelf, Node, nil},
			{Child, &NameTest{"", "emp"}, nil},
		}},
		`*//emp`: &LocationPath{false, []*Step{
			{Child, &NameTest{"", "*"}, nil},
			{DescendantOrSelf, Node, nil},
			{Child, &NameTest{"", "emp"}, nil},
		}},
		`processing-instruction('xsl')`: &LocationPath{false, []*Step{
			{Child, PITest("xsl"), nil},
		}},
		`node()`: &LocationPath{false, []*Step{
			{Child, Node, nil},
		}},
		`text()`: &LocationPath{false, []*Step{
			{Child, Text, nil},
		}},
		`comment()`: &LocationPath{false, []*Step{
			{Child, Comment, nil},
		}},
		`ns1:emp`: &LocationPath{false, []*Step{
			{Child, &NameTest{"ns1", "emp"}, nil},
		}},
		`a:`: &LocationPath{false, []*Step{
			{Child, &NameTest{"a", ""}, nil},
		}},
		`document('test.xml')/*`: &PathExpr{
			&FuncCall{"", "document", []Expr{
				String("test.xml"),
			}},
			&LocationPath{false, []*Step{
				{Child, &NameTest{"", "*"}, nil},
			}},
		},
	}
	for k, v := range tests {
		t.Logf("compiling %s", k)
		expr, err := Parse(k)
		if err != nil {
			t.Errorf("FAIL: %v", err)
			continue
		}
		t.Logf("expr: %v", expr)
		if v != nil && !equals(v, expr) {
			t.Error("FAIL: expr mismatch")
			b, _ := xml.MarshalIndent(expr, "  ", "    ")
			t.Log(string(b))
		}
	}
}

func equals(v1, v2 interface{}) bool {
	switch v1 := v1.(type) {
	case nil:
		return v2 == nil
	case Number, String, NodeType, PITest:
		return v1 == v2
	case *VarRef:
		v2, ok := v2.(*VarRef)
		return ok && *v1 == *v2
	case *NegateExpr:
		v2, ok := v2.(*NegateExpr)
		return ok && equals(v1.Expr, v2.Expr)
	case *BinaryExpr:
		v2, ok := v2.(*BinaryExpr)
		return ok && equals(v1.LHS, v2.LHS) && equals(v1.RHS, v2.RHS) && v1.Op == v2.Op
	case *LocationPath:
		v2, ok := v2.(*LocationPath)
		if !ok || v1.Abs != v2.Abs || len(v1.Steps) != len(v2.Steps) {
			return false
		}
		for i := range v1.Steps {
			if !equals(v1.Steps[i], v2.Steps[i]) {
				return false
			}
		}
		return true
	case *Step:
		v2, ok := v2.(*Step)
		return ok && v1.Axis == v2.Axis && equals(v1.NodeTest, v2.NodeTest) && equals(v1.Predicates, v2.Predicates)
	case *NameTest:
		v2, ok := v2.(*NameTest)
		return ok && *v1 == *v2
	case []Expr:
		v2, ok := v2.([]Expr)
		if !ok || len(v1) != len(v2) {
			return false
		}
		for i := range v1 {
			if !equals(v1[i], v2[i]) {
				return false
			}
		}
		return true
	case *FuncCall:
		v2, ok := v2.(*FuncCall)
		return ok && v1.Prefix == v2.Prefix && v1.Name == v2.Name && equals(v1.Params, v2.Params)
	case *FilterExpr:
		v2, ok := v2.(*FilterExpr)
		return ok && equals(v1.Expr, v2.Expr) && equals(v1.Predicates, v2.Predicates)
	case *PathExpr:
		v2, ok := v2.(*PathExpr)
		return ok && equals(v1.Filter, v2.Filter) && equals(v1.LocationPath, v2.LocationPath)
	default:
		panic(fmt.Sprintf("equals for %T not implemented yet", v1))
	}
}
