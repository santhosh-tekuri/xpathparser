// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xpathparser

import (
	"fmt"
	"strconv"
	"strings"
)

type parser struct {
	lexer  lexer
	tokens []token
	stack  [][]Expr
}

func (p *parser) error(format string, args ...interface{}) error {
	return &Error{fmt.Sprintf(format, args...), p.lexer.xpath, p.token(0).begin}
}

func (p *parser) unexpectedToken() error {
	return p.error("unexpected token %s", p.token(0).kind)
}

func (p *parser) expectedTokens(expected ...kind) error {
	tokens := make([]string, len(expected))
	for i, k := range expected {
		tokens[i] = k.String()
	}
	return p.error("expected %s, but got %v", strings.Join(tokens, " or "), p.token(0).kind)
}

func (p *parser) pushFrame() {
	p.stack = append(p.stack, nil)
}

func (p *parser) peekFrame() []Expr {
	return p.stack[len(p.stack)-1]
}

func (p *parser) popFrame() []Expr {
	frame := p.peekFrame()
	p.stack = p.stack[:len(p.stack)-1]
	return frame
}

func (p *parser) push(expr Expr) {
	p.stack = append(p.stack, append(p.popFrame(), expr))
}

func (p *parser) pushBinary(op Op) {
	p.push(&BinaryExpr{RHS: p.pop(), LHS: p.pop(), Op: op})
}

func (p *parser) pop() Expr {
	frame := p.popFrame()
	v := frame[len(frame)-1]
	p.stack = append(p.stack, frame[:len(frame)-1])
	return v
}

func (p *parser) token(i int) token {
	for i > len(p.tokens)-1 {
		t, err := p.lexer.next()
		if err != nil {
			panic(err)
		}
		p.tokens = append(p.tokens, t)
	}
	return p.tokens[i]
}

func (p *parser) match(k kind) token {
	t := p.token(0)
	if t.kind != k {
		panic(p.error("expected %v, but got %v", k, t.kind))
	}
	p.tokens = p.tokens[1:]
	return t
}

func (p *parser) parse() Expr {
	p.pushFrame()
	p.orExpr()
	p.match(eof)
	return p.pop()
}

func (p *parser) orExpr() {
	p.andExpr()
	if p.token(0).kind == or {
		p.match(or)
		p.orExpr()
		p.pushBinary(Or)
	}
}

func (p *parser) andExpr() {
	p.equalityExpr()
	if p.token(0).kind == and {
		p.match(and)
		p.andExpr()
		p.pushBinary(And)
	}
}

func (p *parser) equalityExpr() {
	p.relationalExpr()
	for {
		switch kind := p.token(0).kind; kind {
		case eq, neq:
			p.match(kind)
			p.relationalExpr()
			p.pushBinary(Op(kind))
		default:
			return
		}
	}
}

func (p *parser) relationalExpr() {
	p.additiveExpr()
	for {
		switch kind := p.token(0).kind; kind {
		case lt, lte, gt, gte:
			p.match(kind)
			p.additiveExpr()
			p.pushBinary(Op(kind))
		default:
			return
		}
	}
}

func (p *parser) additiveExpr() {
	p.multiplicativeExpr()
	for {
		switch kind := p.token(0).kind; kind {
		case plus, minus:
			p.match(kind)
			p.multiplicativeExpr()
			p.pushBinary(Op(kind))
		default:
			return
		}
	}
}

func (p *parser) multiplicativeExpr() {
	p.unaryExpr()
	for {
		switch kind := p.token(0).kind; kind {
		case multiply, div, mod:
			p.match(kind)
			p.unaryExpr()
			p.pushBinary(Op(kind))
		default:
			return
		}
	}
}

func (p *parser) unaryExpr() {
	if p.token(0).kind == minus {
		p.match(minus)
		p.unaryExpr()
		p.push(&NegateExpr{p.pop()})
	} else {
		p.unionExpr()
	}
}

func (p *parser) unionExpr() {
	p.pathExpr()
	if p.token(0).kind == pipe {
		p.match(pipe)
		p.orExpr()
		p.pushBinary(Union)
	}
}

func (p *parser) pathExpr() {
	p.pushFrame()
	switch p.token(0).kind {
	case number, literal:
		p.filterExpr()
		switch p.token(0).kind {
		case slash, slashSlash:
			panic(p.error("nodeset expected"))
		}
	case lparen, dollar:
		p.filterExpr()
		switch p.token(0).kind {
		case slash, slashSlash:
			p.locationPath(false)
		}
	case identifier:
		if (p.token(1).kind == lparen && !isNodeTypeName(p.token(0))) || (p.token(1).kind == colon && p.token(3).kind == lparen) {
			p.filterExpr()
			switch p.token(0).kind {
			case slash, slashSlash:
				p.locationPath(false)
			}
		} else {
			p.locationPath(false)
		}
	case dot, dotDot, star, at:
		p.locationPath(false)
	case slash, slashSlash:
		p.locationPath(true)
	default:
		panic(p.unexpectedToken())
	}

	var path *PathExpr
	if len(p.peekFrame()) == 2 {
		locationPath, ok := p.pop().(*LocationPath)
		if !ok {
			panic("locationPath expected")
		}
		filter, ok := p.pop().(*FilterExpr)
		if !ok {
			panic("filter expected")
		}
		path = &PathExpr{filter, locationPath}
	} else {
		switch v := p.pop().(type) {
		case *LocationPath:
			path = &PathExpr{nil, v}
		case *FilterExpr:
			path = &PathExpr{v, nil}
		default:
			panic("expected locationPath or filter")
		}
	}
	p.popFrame()
	p.push(path)
}

func (p *parser) filterExpr() {
	p.pushFrame()
	switch t := p.token(0); t.kind {
	case number:
		p.match(number)
		f, err := strconv.ParseFloat(t.text(), 64)
		if err != nil {
			panic(err)
		}
		p.push(Number(f))
	case literal:
		p.match(literal)
		p.push(String(t.text()))
	case lparen:
		p.match(lparen)
		p.orExpr()
		p.match(rparen)
	case identifier:
		p.functionCall()
	case dollar:
		p.variableReference()
	}
	p.push(&FilterExpr{p.popFrame()[0], p.predicates()})
}

func (p *parser) functionCall() {
	prefix := ""
	if p.token(1).kind == colon {
		prefix = p.match(identifier).text()
		p.match(colon)
	}
	name := p.match(identifier).text()
	p.pushFrame()
	p.match(lparen)
	p.arguments()
	p.match(rparen)
	p.push(&FuncCall{prefix, name, p.popFrame()})
}

func (p *parser) arguments() {
	for p.token(0).kind != rparen {
		p.orExpr()
		if p.token(0).kind == comma {
			p.match(comma)
		} else {
			break
		}
	}
}

func (p *parser) predicates() []Expr {
	p.pushFrame()
	for p.token(0).kind == lbracket {
		p.match(lbracket)
		p.orExpr()
		p.match(rbracket)
	}
	return p.popFrame()
}

func (p *parser) variableReference() {
	p.match(dollar)
	prefix := ""
	if p.token(1).kind == colon {
		prefix = p.match(identifier).text()
		p.match(colon)
	}
	p.push(&VarRef{prefix, p.match(identifier).text()})
}

func (p *parser) locationPath(abs bool) {
	switch p.token(0).kind {
	case slash, slashSlash:
		if abs {
			p.absoluteLocationPath()
		} else {
			p.relativeLocationPath()
		}
	case at, identifier, dot, dotDot, star:
		p.relativeLocationPath()
	default:
		panic(p.unexpectedToken())
	}
}

func (p *parser) absoluteLocationPath() {
	p.pushFrame()
	switch p.token(0).kind {
	case slash:
		p.match(slash)
		switch p.token(0).kind {
		case dot, dotDot, at, identifier, star:
			p.steps()
		}
	case slashSlash:
		p.push(&Step{DescendantOrSelf, Node, nil})
		p.match(slashSlash)
		switch p.token(0).kind {
		case dot, dotDot, at, identifier, star:
			p.steps()
		default:
			panic(p.error(`locationPath cannot end with "//"`))
		}
	}
	p.push(&LocationPath{true, toSteps(p.popFrame())})
}

func (p *parser) relativeLocationPath() {
	p.pushFrame()
	switch p.token(0).kind {
	case slash:
		p.match(slash)
	case slashSlash:
		p.push(&Step{DescendantOrSelf, Node, nil})
		p.match(slashSlash)
	}
	p.steps()
	p.push(&LocationPath{false, toSteps(p.popFrame())})
}

func (p *parser) steps() {
	switch p.token(0).kind {
	case dot, dotDot, at, identifier, star:
		p.step()
	case eof:
		return
	default:
		panic(p.expectedTokens(dot, dotDot, at, identifier, star))
	}
	for {
		switch p.token(0).kind {
		case slash:
			p.match(slash)
		case slashSlash:
			p.push(&Step{DescendantOrSelf, Node, nil})
			p.match(slashSlash)
		default:
			return
		}
		switch p.token(0).kind {
		case dot, dotDot, at, identifier, star:
			p.step()
		default:
			panic(p.expectedTokens(dot, dotDot, at, identifier, star))
		}
	}
}

func (p *parser) step() {
	var axis Axis
	var nodeTest NodeTest
	switch p.token(0).kind {
	case dot:
		p.match(dot)
		axis, nodeTest = Self, Node
	case dotDot:
		p.match(dotDot)
		axis, nodeTest = Parent, Node
	default:
		switch p.token(0).kind {
		case at:
			p.match(at)
			axis = Attribute
		case identifier:
			if p.token(1).kind == colonColon {
				axis = p.axisSpecifier()
			} else {
				axis = Child
			}
		case star:
			axis = Child
		}
		nodeTest = p.nodeTest(axis)
	}
	p.push(&Step{axis, nodeTest, p.predicates()})
}

func (p *parser) nodeTest(axis Axis) NodeTest {
	switch p.token(0).kind {
	case identifier:
		if p.token(1).kind == lparen {
			return p.nodeTypeTest(axis)
		}
		return p.nameTest(axis)
	case star:
		return p.nameTest(axis)
	default:
		panic(p.expectedTokens(identifier, star))
	}
}

func (p *parser) nodeTypeTest(axis Axis) NodeTest {
	t := p.match(identifier)
	p.match(lparen)
	var nodeTest NodeTest
	switch t.text() {
	case "processing-instruction":
		piName := ""
		if p.token(0).kind == literal {
			piName = p.match(literal).text()
		}
		nodeTest = PITest(piName)
	case "node":
		nodeTest = Node
	case "text":
		nodeTest = Text
	case "comment":
		nodeTest = Comment
	default:
		panic(p.error("invalid nodeType %q", t.text()))
	}
	p.match(rparen)
	return nodeTest
}

func (p *parser) nameTest(axis Axis) NodeTest {
	var prefix string
	if p.token(1).kind == colon && p.token(0).kind == identifier {
		prefix = p.match(identifier).text()
		p.match(colon)
	}
	var localName string
	switch p.token(0).kind {
	case identifier:
		localName = p.match(identifier).text()
	case star:
		p.match(star)
		localName = "*"
	default:
		// let us assume localName as empty-string and continue
	}
	return &NameTest{prefix, localName}
}

func (p *parser) axisSpecifier() Axis {
	name := p.token(0).text()
	axis, ok := name2Axis[name]
	if !ok {
		panic(p.error("invalid axis %s", name))
	}
	p.match(identifier)
	p.match(colonColon)
	return axis
}

func toSteps(frame []Expr) []*Step {
	steps := make([]*Step, len(frame))
	for i := range frame {
		steps[i] = frame[i].(*Step)
	}
	return steps
}

func isNodeTypeName(t token) bool {
	switch t.text() {
	case "node", "comment", "text", "processing-instruction":
		return true
	default:
		return false
	}
}
