// Copyright 2017 Santhosh Kumar Tekuri. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package xpath provides lexer and parser for XPath 1.0.

This Package parses given XPath expression to expression model. 

To Evaluate XPath, use https://github.com/santhosh-tekuri/xpatheng

	expr, err := xpath.Parse("(/a/b)[5]")
	if err != nil {
  		fmt.Println(err)
  		return
	}
	fmt.Println(expr)

*/
package xpath
