### Go function binding



```
ctx.SetFunc("goFunction", func(this *qjs.This) (*qjs.Value, error) {
    return this.Context().NewString("Hello from Go!"), nil
})

result, err := ctx.Eval("test.js", qjs.Code(`
	const message = goFunction();
	message;
`))
if err != nil {
	log.Fatal("Eval error:", err)
}
defer result.Free()

// Output: Hello from Go!
log.Println(result.String())
```



### HTTP Handlers in JavaScript



```
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/fastschema/qjs"
)

func must[T any](val T, err error) T {
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	return val
}

const script = `
// JS handlers for HTTP routes
const about = () => {
	return "QuickJS in Go - Hello World!";
};

const contact = () => {
	return "Contact us at contact@example.com";
};

export default { about, contact };
`

func main() {
	rt := must(qjs.New())
	defer rt.Close()
	ctx := rt.Context()

	// Precompile the script to bytecode
	byteCode := must(ctx.Compile("script.js", qjs.Code(script), qjs.TypeModule()))
	// Use a pool of runtimes for concurrent requests
	pool := qjs.NewPool(3, &qjs.Option{}, func(r *qjs.Runtime) error {
		results := must(r.Context().Eval("script.js", qjs.Bytecode(byteCode), qjs.TypeModule()))
		// Store the exported functions in the global object for easy access
		r.Context().Global().SetPropertyStr("handlers", results)
		return nil
	})

	// Register HTTP handlers based on JS functions
	val := must(ctx.Eval("script.js", qjs.Bytecode(byteCode), qjs.TypeModule()))
	methodNames := must(val.GetOwnPropertyNames())
	val.Free()
	for _, methodName := range methodNames {
		http.HandleFunc("/"+methodName, func(w http.ResponseWriter, r *http.Request) {
			runtime := must(pool.Get())
			defer pool.Put(runtime)

			// Call the corresponding JS function
			handlers := runtime.Context().Global().GetPropertyStr("handlers")
			result := must(handlers.InvokeJS(methodName))
			fmt.Fprint(w, result.String())
			result.Free()
		})
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from Go's HTTP server!")
	})

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}
```



### Async operations



**Awaiting a promise**

```
ctx.SetAsyncFunc("asyncFunction", func(this *qjs.This) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		result := this.Context().NewString("Async result from Go!")
		this.Promise().Resolve(result)
	}()
})

result, err := ctx.Eval("test.js", qjs.Code(`
async function main() {
	const result = await asyncFunction();
	return result;
}
({ main: main() });
`))

if err != nil {
	log.Fatal("Eval error:", err)
}
defer result.Free()

mainFunc := result.GetPropertyStr("main")

// Wait for the promise to resolve
val, err := mainFunc.Await()
if err != nil {
	log.Fatal("Await error:", err)
}

// Output: Async result from Go!
log.Println("Awaited value:", val.String())
```



**Top level await**

```
// asyncFunction is already defined above
result, err := ctx.Eval("test.js", qjs.Code(`
	async function main() {
		const result = await asyncFunction();
		return result;
	}
	await main()
`), qjs.FlagAsync())

if err != nil {
	log.Fatal("Eval error:", err)
}

defer result.Free()
log.Println(result.String())
```



### Call JS function from Go



```
// Call JS function from Go
result, err := ctx.Eval("test.js", qjs.Code(`
	function add(a, b) {
		return a + b;
	}

	function errorFunc() {
		throw new Error("test error");
	}

	({
		addFunc: add,
		errorFunc: errorFunc
	});
`))

if err != nil {
	log.Fatal("Eval error:", err)
}
defer result.Free()

jsAddFunc := result.GetPropertyStr("addFunc")
defer jsAddFunc.Free()

goAddFunc, err := qjs.JsFuncToGo[func(int, int) (int, error)](jsAddFunc)
if err != nil {
	log.Fatal("Func conversion error:", err)
}

total, err := goAddFunc(1, 2)
if err != nil {
	log.Fatal("Func execution error:", err)
}

// Output: 3
log.Println("Addition result:", total)

jsErrorFunc := result.GetPropertyStr("errorFunc")
defer jsErrorFunc.Free()

goErrorFunc, err := qjs.JsFuncToGo[func() (any, error)](jsErrorFunc)
if err != nil {
	log.Fatal("Func conversion error:", err)
}

_, err = goErrorFunc()
if err != nil {
	// Output:
	// JS function execution failed: Error: test error
  //  at errorFunc (test.js:7:13)
	log.Println(err.Error())
}
```



### ES Modules



```
// Load a utility module
if _, err = ctx.Load("math-utils.js", qjs.Code(`
	export function add(a, b) {
		return a + b;
	}

	export function multiply(a, b) {
		return a * b;
	}

	export function power(base, exponent) {
		return Math.pow(base, exponent);
	}

	export const PI = 3.14159;
	export const E = 2.71828;
	export default {
		add,
		multiply,
		power,
		PI,
		E
	};
`)); err != nil {
	log.Fatal("Module load error:", err)
}

// Use the module
result, err := ctx.Eval("use-math.js", qjs.Code(`
	import mathUtils, { add, multiply, power, PI } from 'math-utils.js';

	const calculations = {
		addition: add(10, 20),
		multiplication: multiply(6, 7),
		power: power(2, 8),
		circleArea: PI * power(5, 2),
		defaultAdd: mathUtils.add(10, 20)
	};

	export default calculations;
`), qjs.TypeModule())

if err != nil {
	log.Fatal("Module eval error:", err)
}

// Output:
// Addition: 30
// Multiplication: 42
// Power: 256
// Circle Area: 78.54
// Default Add: 30
fmt.Printf("Addition: %d\n", result.GetPropertyStr("addition").Int32())
fmt.Printf("Multiplication: %.0f\n", result.GetPropertyStr("multiplication").Float64())
fmt.Printf("Power: %.0f\n", result.GetPropertyStr("power").Float64())
fmt.Printf("Circle Area: %.2f\n", result.GetPropertyStr("circleArea").Float64())
fmt.Printf("Default Add: %.d\n", result.GetPropertyStr("defaultAdd").Int32())
result.Free()
```



### Bytecode Compilation



```
script := `
	function fibonacci(n) {
		if (n <= 1) return n;
		return fibonacci(n - 1) + fibonacci(n - 2);
	}

	function factorial(n) {
		return n <= 1 ? 1 : n * factorial(n - 1);
	}

	const result = {
		fib10: fibonacci(10),
		fact5: factorial(5),
		timestamp: Date.now()
	};

	result;
`

// Compile the script to bytecode
bytecode, err := ctx.Compile("math-functions.js", qjs.Code(script))
if err != nil {
	log.Fatal("Compilation error:", err)
}

fmt.Printf("Bytecode size: %d bytes\n", len(bytecode))

// Execute the compiled bytecode
result, err := ctx.Eval("compiled-math.js", qjs.Bytecode(bytecode))
if err != nil {
	log.Fatal("Bytecode execution error:", err)
}

fmt.Printf("Fibonacci(10): %d\n", result.GetPropertyStr("fib10").Int32())
fmt.Printf("Factorial(5): %d\n", result.GetPropertyStr("fact5").Int32())
result.Free()
```



### ProxyValue Support



ProxyValue is a feature that allows you to pass Go values directly to JavaScript without full serialization, enabling efficient sharing of complex objects, functions, and resources.

ProxyValue creates a lightweight JavaScript wrapper around Go values, storing only a reference ID rather than copying the entire value. This is particularly useful for **pass-through scenarios** where JavaScript receives a Go value and passes it back to Go without needing to access its contents.

Key benefits:

- **Zero-copy data sharing** - no serialization/deserialization overhead.
- **Pass-through efficiency** - JavaScript can hold and return Go values without conversion.
- **Type preservation** - original Go types are maintained across boundaries.
- **Resource efficiency** - perfect for objects like `context.Context`, database connections, or large structs.

#### Basic ProxyValue Usage



```
// Create a Go function that accepts context and a number
goFuncWithContext := func(c context.Context, num int) int {
	// Access context values in Go
	log.Println("Context value:", c.Value("key"))
	return num * 2
}

// Convert Go function to JavaScript function
jsFuncWithContext, err := qjs.ToJSValue(ctx, goFuncWithContext)
if err != nil {
	log.Fatal("Func conversion error:", err)
}
defer jsFuncWithContext.Free()
ctx.Global().SetPropertyStr("funcWithContext", jsFuncWithContext)

// Create a helper function that returns a ProxyValue
ctx.SetFunc("$context", func(this *qjs.This) (*qjs.Value, error) {
	// Create context as ProxyValue - JavaScript will never access its contents
	passContext := context.WithValue(context.Background(), "key", "value123")
	val := ctx.NewProxyValue(passContext)
	return val, nil
})

// JavaScript gets context as ProxyValue and passes it to Go function
result, err := ctx.Eval("test.js", qjs.Code(`
	funcWithContext($context(), 10);
`))
if err != nil {
	log.Fatal("Eval error:", err)
}
defer result.Free()

// Output: 20
log.Println("Result:", result.Int32())
```



### GO-JS Conversion



```
package main

import (
	"fmt"
	"log"

	"github.com/fastschema/qjs"
)

type Post struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Author User   `json:"author"`
}

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Method on User struct
func (u User) GetDisplayName() string {
	return fmt.Sprintf("%s (%d)", u.Name, u.Age)
}

func (u User) IsAdult() bool {
	return u.Age >= 18
}

func main() {
	rt, err := qjs.New()
	if err != nil {
		log.Fatalf("Failed to create QuickJS runtime: %v", err)
	}
	defer rt.Close()
	ctx := rt.Context()

	ctx.Global().SetPropertyStr("goInt", ctx.NewInt32(55))
	ctx.Global().SetPropertyStr("goString", ctx.NewString("Hello, World!"))
	jsUser, err := qjs.ToJSValue(ctx, User{ID: 1, Name: "Alice", Age: 25})
	if err != nil {
		log.Fatalf("Failed to convert User to JS value: %v", err)
	}
	ctx.Global().SetPropertyStr("goUser", jsUser)

	result, err := ctx.Eval("test.js", qjs.Code(`
		const post = {
			id: goInt,
			name: goString,
			author: goUser,
			displayName: goUser.GetDisplayName(),
			isAdult: goUser.IsAdult()
		};
		post;
	`))
	if err != nil {
		log.Fatalf("Failed to evaluate JS code: %v", err)
	}
	defer result.Free()

	goPost, err := qjs.JsValueToGo[Post](result)
	if err != nil {
		log.Fatalf("Failed to convert JS value to Post: %v", err)
	}

	// Output:
	// Post ID: 55
	// Post Name: Hello, World!
	// Author ID: 1
	// Author Name: Alice
	// Author Age: 25
	// Author Display Name: Alice (25)
	// Author Is Adult: true
	log.Printf("Post ID: %d\n", goPost.ID)
	log.Printf("Post Name: %s\n", goPost.Name)
	log.Printf("Author ID: %d\n", goPost.Author.ID)
	log.Printf("Author Name: %s\n", goPost.Author.Name)
	log.Printf("Author Age: %d\n", goPost.Author.Age)
	log.Printf("Author Display Name: %s\n", goPost.Author.GetDisplayName())
	log.Printf("Author Is Adult: %t\n", goPost.Author.IsAdult())
}
```



### Pool



```
package main

import (
	"log"
	"sync"

	"github.com/fastschema/qjs"
)

func main() {
	setupFunc := func(rt *qjs.Runtime) error {
		ctx := rt.Context()
		ctx.Eval("setup.js", qjs.Code(`
			function getMessage(workerId, taskId) {
				return "Hello from pooled runtime: " + workerId + "-" + taskId;
			}
		`))
		return nil
	}
	// Create a pool with 3 runtimes
	pool := qjs.NewPool(3, &qjs.Option{}, setupFunc)
	numWorkers := 5
	numTasks := 3
	var wg sync.WaitGroup

	for i := range numWorkers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numTasks; j++ {
				rt, err := pool.Get()
				if err != nil {
					log.Fatalf("Failed to get runtime from pool: %v", err)
				}
				defer pool.Put(rt)
				ctx := rt.Context()
				workerIdValue := ctx.NewInt32(int32(workerID))
				taskIdValue := ctx.NewInt32(int32(j))
				ctx.Global().SetPropertyStr("workerID", workerIdValue)
				ctx.Global().SetPropertyStr("taskID", taskIdValue)

				// Use the runtime
				result, err := ctx.Eval("pool-test.js", qjs.Code(`
					({
						message: getMessage(workerID, taskID),
						timestamp: Date.now(),
					});
				`))
				if err != nil {
					log.Fatalf("JS execution error: %v", err)
				}
				defer result.Free()
				log.Println(result.GetPropertyStr("message").String())
			}
		}(i)
	}
	wg.Wait()
}
```
