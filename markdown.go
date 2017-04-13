package main

import (
	"fmt"

	"github.com/russross/blackfriday"
)

const data = `
hello
-------

Relative links to files/images just work.

<img src="img.jpg" width="600px"/>

Do you like my above inline image?
and here is a list:

* one
* two
* three
`

func renderHtml() {
	fmt.Printf("%s", blackfriday.MarkdownCommon(input))
}
