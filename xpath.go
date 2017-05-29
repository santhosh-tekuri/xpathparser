// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xpath

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

type Error struct {
	Msg    string
	XPath  string
	Offset int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s in xpath %s at offset %d", e.Msg, e.XPath, e.Offset)
}

type Axis int

const (
	Child Axis = iota
	Descendant
	Parent
	Ancestor
	FollowingSibling
	PrecedingSibling
	Following
	Preceding
	Attribute
	Namespace
	Self
	DescendantOrSelf
	AncestorOrSelf
)

var axisNames = []string{
	"child",
	"descendant",
	"parent",
	"ancestor",
	"following-sibling",
	"preceding-sibling",
	"following",
	"preceding",
	"attribute",
	"namespace",
	"self",
	"descendant-or-self",
	"ancestor-or-self",
}

func (a Axis) String() string {
	return axisNames[a]
}

var name2Axis = make(map[string]Axis)

func init() {
	for i, name := range axisNames {
		name2Axis[name] = Axis(i)
	}
}

type NodeType int

const (
	Comment NodeType = iota
	Text
	Node
)

func (nt NodeType) String() string {
	switch nt {
	case Comment:
		return "comment()"
	case Text:
		return "text()"
	case Node:
		return "node()"
	}
	panic(fmt.Sprintf("unknown NodeType %d", int(nt)))
}

type Op int

const (
	EQ Op = iota
	NEQ
	LT
	LTE
	GT
	GTE
	Add
	Subtract
	Multiply
	Mod
	Div
	And
	Or
	Union
)

var opNames = []string{"=", "!=", "<", "<=", ">", ">=", "+", "-", "*", "mod", "div", "and", "or", "|"}

func (op Op) String() string {
	return opNames[op]
}

// An Expr is an interface holding one of the types:
// *LocationPath, *Binary, *Negate, *FilterExpr, *PathExpr, *VarRef, *FuncCall, Number, String or Boolean.
type Expr interface{}

type BinaryExpr struct {
	LHS Expr
	Op  Op
	RHS Expr
}

func (b *BinaryExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", b.LHS, b.Op, b.RHS)
}

type NegateExpr struct {
	Expr Expr
}

func (n *NegateExpr) String() string {
	return fmt.Sprintf("-%s", n.Expr)
}

type LocationPath struct {
	Abs   bool
	Steps []*Step
}

func (l *LocationPath) String() string {
	s := make([]string, len(l.Steps))
	for i, step := range l.Steps {
		s[i] = step.String()
	}
	if l.Abs {
		return fmt.Sprintf("/%s", strings.Join(s, "/"))
	}
	return fmt.Sprintf("%s", strings.Join(s, "/"))
}

type FilterExpr struct {
	Expr       Expr
	Predicates []Expr
}

func (f *FilterExpr) String() string {
	return fmt.Sprintf("%s%s", f.Expr, predicatesString(f.Predicates))
}

type PathExpr struct {
	Filter       Expr // can be nil
	LocationPath *LocationPath
}

func (p *PathExpr) String() string {
	s := ""
	if p.Filter != nil {
		s += fmt.Sprint(p.Filter)
	}
	if p.LocationPath != nil {
		if len(s) > 0 {
			s = "(" + s + ")"
		}
		if len(p.LocationPath.Steps) > 0 {
			s += "/"
		}
		s += p.LocationPath.String()
	}
	return s
}

type Step struct {
	Axis       Axis
	NodeTest   NodeTest
	Predicates []Expr
}

func (s *Step) String() string {
	return fmt.Sprintf("%v::%s%s", s.Axis, s.NodeTest, predicatesString(s.Predicates))
}

// A NodeTest is an interface holding one of the types:
// NodeType, *NameTest, or PITest.
type NodeTest interface{}

type NameTest struct {
	Prefix string
	Local  string
}

func (nt *NameTest) String() string {
	if nt.Prefix == "" {
		return nt.Local
	}
	return fmt.Sprintf("%s:%s", nt.Prefix, nt.Local)
}

type PITest string

func (pt PITest) String() string {
	return fmt.Sprintf("processing-instruction(%q)", string(pt))
}

type VarRef struct {
	Prefix string
	Local  string
}

func (vr *VarRef) String() string {
	if vr.Prefix == "" {
		return fmt.Sprintf("$%s", vr.Local)
	}
	return fmt.Sprintf("$%s:%s", vr.Prefix, vr.Local)
}

type FuncCall struct {
	Prefix string
	Name   string
	Params []Expr
}

func (fc *FuncCall) String() string {
	p := make([]string, len(fc.Params))
	for i, param := range fc.Params {
		p[i] = fmt.Sprint(param)
	}
	if fc.Prefix == "" {
		return fmt.Sprintf("%s(%s)", fc.Name, strings.Join(p, ", "))
	}
	return fmt.Sprintf("%s:%s(%s)", fc.Prefix, fc.Name, strings.Join(p, ", "))
}

type Number float64

func (n Number) String() string {
	return strconv.FormatFloat(float64(n), 'f', -1, 64)
}

type String string

func (l String) String() string {
	return strconv.Quote(string(l))
}

func MustParse(xpath string) Expr {
	p := &parser{lexer: lexer{xpath: xpath}}
	return simplify(p.parse())
}

func Parse(xpath string) (expr Expr, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	p := &parser{lexer: lexer{xpath: xpath}}
	return simplify(p.parse()), nil
}

func predicatesString(predicates []Expr) string {
	p := make([]string, len(predicates))
	for i, predicate := range predicates {
		p[i] = fmt.Sprintf("[%s]", predicate)
	}
	return strings.Join(p, "")
}

func simplify(e Expr) Expr {
	switch e := e.(type) {
	case *BinaryExpr:
		e.LHS = simplify(e.LHS)
		e.RHS = simplify(e.RHS)
		return e
	case *NegateExpr:
		e.Expr = simplify(e.Expr)
		return e
	case *PathExpr:
		if e.Filter != nil {
			e.Filter = simplify(e.Filter)
		}
		if e.LocationPath != nil {
			e.LocationPath = simplify(e.LocationPath).(*LocationPath)
		}
		if e.Filter == nil {
			return e.LocationPath
		}
		if e.LocationPath == nil {
			return e.Filter
		}
		return e
	case *FilterExpr:
		e.Expr = simplify(e.Expr)
		if len(e.Predicates) == 0 {
			return e.Expr
		}
		for i, predicate := range e.Predicates {
			e.Predicates[i] = simplify(predicate)
		}
		return e
	case *LocationPath:
		for _, step := range e.Steps {
			for i, predicate := range step.Predicates {
				step.Predicates[i] = simplify(predicate)
			}
		}
		return e
	case *FuncCall:
		for i, p := range e.Params {
			e.Params[i] = simplify(p)
		}
		return e
	}
	return e
}
