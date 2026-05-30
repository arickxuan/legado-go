package tool

import (
	"log"

	"github.com/fastschema/qjs"
)

func TestJs2() {

	rt, err := qjs.New()
	if err != nil {
		log.Fatal(err)
	}

	defer rt.Close()
	ctx := rt.Context()

	result, err := ctx.Eval("test.js", qjs.Code(`
	const person = {
		name: "Alice",
		age: 30,
		city: "New York"
	};

	const info = Object.keys(person).map(key =>
		key + ": " + person[key]
	).join(", ");

	// The last expression is the return value
	({ person: person, info: info });
`))
	if err != nil {
		log.Fatal("Eval error:", err)
	}
	defer result.Free()
	// Output: name: Alice, age: 30, city: New York
	log.Println(result.GetPropertyStr("info").String())
	// Output: Alice
	log.Println(result.GetPropertyStr("person").GetPropertyStr("name").String())
	// Output: 30
	log.Println(result.GetPropertyStr("person").GetPropertyStr("age").Int32())

}
