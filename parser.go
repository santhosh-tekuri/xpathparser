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

func (p *parser) unexpectedToken() {
	t := p.token(0)
	panic(&Error{fmt.Sprintf("unexpected token %s", t.kind), p.lexer.xpath, t.begin})
}

func (p *parser) expectedTokens(expected ...kind) {
	tokens := make([]string, len(expected))
	for i, k := range expected {
		tokens[i] = k.String()
	}
	t := p.token(0)
	panic(&Error{fmt.Sprintf("expected %s, but got %v", strings.Join(tokens, " or "), t.kind), p.lexer.xpath, t.begin})
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
		panic(&Error{fmt.Sprintf("expected %v, but got %v", k, t.kind), p.lexer.xpath, t.begin})
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
	t := p.token(0)
	for t.kind == eq || t.kind == neq {
		p.match(t.kind)
		p.relationalExpr()
		if t.kind == eq {
			p.pushBinary(EQ)
		} else {
			p.pushBinary(NEQ)
		}
		t = p.token(0)
	}
}

func (p *parser) relationalExpr() {
	p.additiveExpr()
	t := p.token(0)
	for t.kind == lt || t.kind == gt || t.kind == lte || t.kind == gte {
		p.match(t.kind)
		p.additiveExpr()
		switch t.kind {
		case lt:
			p.pushBinary(LT)
		case gt:
			p.pushBinary(GT)
		case lte:
			p.pushBinary(LTE)
		case gte:
			p.pushBinary(GTE)
		}
		t = p.token(0)
	}
}

func (p *parser) additiveExpr() {
	p.multiplicativeExpr()
	t := p.token(0)
	for t.kind == plus || t.kind == minus {
		p.match(t.kind)
		p.multiplicativeExpr()
		if t.kind == plus {
			p.pushBinary(Add)
		} else {
			p.pushBinary(Subtract)
		}
		t = p.token(0)
	}
}

func (p *parser) multiplicativeExpr() {
	p.unaryExpr()
	t := p.token(0)
	for t.kind == multiply || t.kind == div || t.kind == mod {
		p.match(t.kind)
		p.unaryExpr()
		switch t.kind {
		case multiply:
			p.pushBinary(Multiply)
		case div:
			p.pushBinary(Div)
		case mod:
			p.pushBinary(Mod)
		}
		t = p.token(0)
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
	t := p.token(0)
	switch t.kind {
	case number, literal:
		p.filterExpr()
		t = p.token(0)
		if t.kind == slash || t.kind == slashSlash {
			panic("nodeset expected")
		}
	case lparen, dollar:
		p.filterExpr()
		t = p.token(0)
		if t.kind == slash || t.kind == slashSlash {
			p.locationPath(false)
		}
	case identifier:
		if (p.token(1).kind == lparen && !isNodeTypeName(p.token(0))) || (p.token(1).kind == colon && p.token(3).kind == lparen) {
			p.filterExpr()
			t = p.token(0)
			if t.kind == slash || t.kind == slashSlash {
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
		p.unexpectedToken()
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
		v := p.pop()
		if lp, ok := v.(*LocationPath); ok {
			path = &PathExpr{nil, lp}
		} else if f, ok := v.(*FilterExpr); ok {
			path = &PathExpr{f, nil}
		} else {
			panic("expected locationPath or filter")
		}
	}
	p.popFrame()
	p.push(path)
}

func (p *parser) filterExpr() {
	p.pushFrame()
	t := p.token(0)
	switch t.kind {
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
	p.predicates()
	frame := p.popFrame()
	filter := &FilterExpr{frame[0], make([]Expr, len(frame)-1)}
	for i := range filter.Predicates {
		filter.Predicates[i] = frame[i+1]
	}
	p.push(filter)
}

func (p *parser) functionCall() {
	prefix := ""
	if p.token(1).kind == colon {
		prefix = p.match(identifier).text()
		p.match(colon)
	}
	name := p.match(identifier).text()
	p.pushFrame()
	p.push(&FuncCall{prefix, name, nil})
	p.match(lparen)
	p.arguments()
	p.match(rparen)
	frame := p.popFrame()
	fcall := frame[0].(*FuncCall)
	fcall.Args = make([]Expr, len(frame)-1)
	for i := range fcall.Args {
		fcall.Args[i] = frame[i+1]
	}
	p.push(fcall)
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

func (p *parser) predicates() {
	for p.token(0).kind == lbracket {
		p.pushFrame()
		p.match(lbracket)
		p.orExpr()
		p.match(rbracket)
		predicate := p.pop()
		p.popFrame()
		p.push(predicate)
	}
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
		p.unexpectedToken()
	}
}

func (p *parser) absoluteLocationPath() {
	p.pushFrame()
	p.push(&LocationPath{true, nil})
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
			panic(&Error{`locationPath cannot end with "//"`, p.lexer.xpath, p.token(0).begin})
		}
	}
	p.endLocationPath()
}

func (p *parser) relativeLocationPath() {
	p.pushFrame()
	p.push(&LocationPath{false, nil})
	switch p.token(0).kind {
	case slash:
		p.match(slash)
	case slashSlash:
		p.push(&Step{DescendantOrSelf, Node, nil})
		p.match(slashSlash)
	}
	p.steps()
	p.endLocationPath()
}

func (p *parser) steps() {
	switch p.token(0).kind {
	case dot, dotDot, at, identifier, star:
		p.step()
	case eof:
		return
	default:
		p.expectedTokens(dot, dotDot, at, identifier, star)
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
			p.expectedTokens(dot, dotDot, at, identifier, star)
		}
	}
}

func (p *parser) step() {
	switch p.token(0).kind {
	case dot:
		p.match(dot)
		p.pushFrame()
		p.push(&Step{Self, Node, nil})
		p.predicates()
		p.endStep()
	case dotDot:
		p.match(dotDot)
		p.pushFrame()
		p.push(&Step{Parent, Node, nil})
		p.predicates()
		p.endStep()
	default:
		var axis Axis
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
		p.nodeTest(axis)
	}
}

func (p *parser) nodeTest(axis Axis) {
	switch p.token(0).kind {
	case identifier:
		if p.token(1).kind == lparen {
			p.nodeTypeTest(axis)
		} else {
			p.nameTest(axis)
		}
	case star:
		p.nameTest(axis)
	default:
		p.expectedTokens(identifier, star)
	}
}

func (p *parser) nodeTypeTest(axis Axis) {
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
		panic(&Error{fmt.Sprintf("invalid nodeType %q", t.text()), p.lexer.xpath, t.begin})
	}
	p.match(rparen)
	p.pushFrame()
	p.push(&Step{axis, nodeTest, nil})
	p.predicates()
	p.endStep()
}

func (p *parser) nameTest(axis Axis) {
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
	p.pushFrame()
	p.push(&Step{axis, &NameTest{prefix, localName}, nil})
	p.predicates()
	p.endStep()
}

func (p *parser) axisSpecifier() Axis {
	name := p.token(0).text()
	axis, ok := name2Axis[name]
	if !ok {
		panic(&Error{"invalid axis " + name, p.lexer.xpath, p.token(0).begin})
	}
	p.match(identifier)
	p.match(colonColon)
	return axis
}

func (p *parser) endStep() {
	frame := p.popFrame()
	step := frame[0].(*Step)
	step.Predicates = make([]Expr, len(frame)-1)
	for i := range step.Predicates {
		step.Predicates[i] = frame[i+1]
	}
	p.push(step)
}

func (p *parser) endLocationPath() {
	frame := p.popFrame()
	lpath := frame[0].(*LocationPath)
	lpath.Steps = make([]*Step, len(frame)-1)
	for i := range lpath.Steps {
		lpath.Steps[i] = frame[i+1].(*Step)
	}
	p.push(lpath)
}

func isNodeTypeName(t token) bool {
	switch t.text() {
	case "node", "comment", "text", "processing-instruction":
		return true
	default:
		return false
	}
}
