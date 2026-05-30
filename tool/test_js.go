package tool

// func EvelJsTest() {
// 	runtime.LockOSThread()
// 	defer runtime.UnlockOSThread()
// 	opt := quickjs.WithExecuteTimeout(30)

// 	// 创建新的运行时
// 	rt := quickjs.NewRuntime(opt)
// 	defer rt.Close()

// 	// 创建新的上下文
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	ret := ctx.Eval("'Hello ' + 'QuickJS!'")
// 	defer ret.Free()

// 	if ret.IsException() {
// 		err := ctx.Exception()
// 		println(err.Error())
// 		return
// 	}

// 	fmt.Println(ret.ToString())
// }

// func JsSetTest() {
// 	// 创建新的运行时
// 	rt := quickjs.NewRuntime()
// 	defer rt.Close()

// 	// 创建新的上下文
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	test := ctx.NewObject()
// 	test.Set("A", ctx.NewString("String A"))
// 	test.Set("B", ctx.NewString("String B"))
// 	test.Set("C", ctx.NewString("String C"))
// 	ctx.Globals().Set("test", test)

// 	ret := ctx.Eval(`Object.keys(test).map(key => test[key]).join(" ")`)
// 	defer ret.Free()

// 	if ret.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}

// 	fmt.Println(ret.ToString())
// }

// func BindTest() {
// 	// 创建新的运行时
// 	rt := quickjs.NewRuntime()
// 	defer rt.Close()

// 	// 创建新的上下文
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	// 创建新对象
// 	test := ctx.NewObject()
// 	// 将属性绑定到对象
// 	test.Set("A", ctx.NewString("String A"))
// 	test.Set("B", ctx.NewInt32(0))
// 	test.Set("C", ctx.NewBool(false))
// 	// 将 go 函数绑定到 js 对象
// 	test.Set("hello", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
// 		return ctx.NewString("Hello " + args[0].ToString())
// 	}))

// 	// 将 "test" 对象绑定到全局对象
// 	ctx.Globals().Set("test", test)

// 	// 通过 js 调用 js 函数（同步）
// 	js_ret := ctx.Eval(`test.hello("Javascript!")`)
// 	defer js_ret.Free()
// 	if js_ret.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}
// 	fmt.Println(js_ret.ToString())

// 	// 通过 go 调用 js 函数（同步）
// 	go_ret := test.Call("hello", ctx.NewString("Golang!"))
// 	defer go_ret.Free()
// 	if go_ret.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}
// 	fmt.Println(go_ret.ToString())

// 	// --- 同步 Promise 示例（立即 resolve/reject，不开 goroutine）---
// 	ctx.Globals().Set("syncPromise", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
// 		return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
// 			// 可以在 executor 中直接同步 resolve：
// 			msg := ctx.NewString("Hello from sync Promise")
// 			defer msg.Free()
// 			resolve(msg)

// 			// 如需同步失败，可以改为：
// 			// errVal := ctx.NewString("sync error")
// 			// defer errVal.Free()
// 			// reject(errVal)
// 		})
// 	}))

// 	syncPromise := ctx.Eval(`syncPromise()`)
// 	defer syncPromise.Free()

// 	syncRet := ctx.Await(syncPromise)
// 	defer syncRet.Free()
// 	if syncRet.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}
// 	fmt.Println(syncRet.ToString())

// 	// --- 使用 Function + Promise + 调度器 的异步示例 ---
// 	ctx.Globals().Set("testAsync", ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
// 		return ctx.NewPromise(func(resolve, reject func(*quickjs.Value)) {
// 			// 耗时或阻塞操作可以放在 goroutine 中
// 			go func() {
// 				time.Sleep(10 * time.Millisecond)

// 				// 但 QuickJS/Context API 必须回到 Context 线程上
// 				// 通过 ctx.Schedule 调度执行
// 				ctx.Schedule(func(inner *quickjs.Context) {
// 					value := inner.NewString("Hello Async Function!")
// 					defer value.Free()
// 					resolve(value)
// 				})
// 			}()
// 		})
// 	}))

// 	// 从 JS 角度看，这只是一个返回 Promise 的普通函数
// 	promiseResult := ctx.Eval(`testAsync()`)
// 	defer promiseResult.Free()

// 	// ctx.Await 内部会驱动 QuickJS 的 pending-job 队列
// 	// 以及 Context 级调度器，因此如果你只关心 Promise
// 	// 的最终结果，通常不需要单独调用 ctx.Loop()
// 	asyncRet := ctx.Await(promiseResult)
// 	defer asyncRet.Free()

// 	if asyncRet.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}

// 	fmt.Println(asyncRet.ToString())

// 	// 输出:
// 	// Hello Javascript!
// 	// Hello Golang!
// 	// Hello from sync Promise
// 	// Hello Async Function!
// }

// // 注意：
// // - 线程归属由调用方负责；如果一个 Runtime 必须固定在同一个 OS 线程，请由调用方自行在 owner goroutine 中调用 runtime.LockOSThread()。
// // - WithOwnerGoroutineCheck(false) 是不安全开关，只应在你能保证外部串行化 QuickJS 访问时使用。
// // - 不要在 goroutine 中直接调用 Context 或任何 QuickJS API。
// // - 所有 QuickJS 相关操作都应该通过 ctx.Schedule 调度回 Context 线程后再执行。
// // - ctx.Await 会在内部驱动 pending jobs 和调度器，直至 Promise 解决。

// func JsToGo() {
// 	rt := quickjs.NewRuntime()
// 	defer rt.Close()
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	// 在 JavaScript 中创建 TypedArray
// 	jsTypedArrays := ctx.Eval(`
//         ({
//             int8: new Int8Array([-128, -1, 0, 1, 127]),
//             uint16: new Uint16Array([0, 32768, 65535]),
//             float64: new Float64Array([Math.PI, Math.E, 42.5]),
//             bigUint64: new BigUint64Array([0n, 18446744073709551615n])
//         })
//     `)
// 	defer jsTypedArrays.Free()

// 	if jsTypedArrays.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}

// 	// 转换为 Go 切片
// 	int8Array := jsTypedArrays.Get("int8")
// 	defer int8Array.Free()
// 	if int8Array.IsInt8Array() {
// 		goInt8Slice, err := int8Array.ToInt8Array()
// 		if err == nil {
// 			fmt.Printf("Int8Array: %v\n", goInt8Slice)
// 		}
// 	}

// 	uint16Array := jsTypedArrays.Get("uint16")
// 	defer uint16Array.Free()
// 	if uint16Array.IsUint16Array() {
// 		goUint16Slice, err := uint16Array.ToUint16Array()
// 		if err == nil {
// 			fmt.Printf("Uint16Array: %v\n", goUint16Slice)
// 		}
// 	}

// 	float64Array := jsTypedArrays.Get("float64")
// 	defer float64Array.Free()
// 	if float64Array.IsFloat64Array() {
// 		goFloat64Slice, err := float64Array.ToFloat64Array()
// 		if err == nil {
// 			fmt.Printf("Float64Array: %v\n", goFloat64Slice)
// 		}
// 	}

// 	bigUint64Array := jsTypedArrays.Get("bigUint64")
// 	defer bigUint64Array.Free()
// 	if bigUint64Array.IsBigUint64Array() {
// 		goBigUint64Slice, err := bigUint64Array.ToBigUint64Array()
// 		if err == nil {
// 			fmt.Printf("BigUint64Array: %v\n", goBigUint64Slice)
// 		}
// 	}
// }

// func MarshalTest() {
// 	rt := quickjs.NewRuntime()
// 	defer rt.Close()
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	// 将 Go 值 Marshal 为 JavaScript 值
// 	data := map[string]interface{}{
// 		"name":   "张三",
// 		"age":    30,
// 		"active": true,
// 		"scores": []int{85, 92, 78},
// 		"address": map[string]string{
// 			"city":    "北京",
// 			"country": "中国",
// 		},
// 		// 类型化切片将自动创建 TypedArray
// 		"floatData": []float32{1.1, 2.2, 3.3},
// 		"intData":   []int32{100, 200, 300},
// 		"byteData":  []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}, // "Hello" 的字节
// 	}

// 	jsVal, err := ctx.Marshal(data)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer jsVal.Free()

// 	// 在 JavaScript 中使用 marshal 的值
// 	ctx.Globals().Set("user", jsVal)
// 	result := ctx.Eval(`
//         const info = user.name + " 今年 " + user.age + " 岁";
//         const floatArrayType = user.floatData instanceof Float32Array;
//         const intArrayType = user.intData instanceof Int32Array;
//         const byteArrayType = user.byteData instanceof ArrayBuffer;

//         ({
//             info: info,
//             floatArrayType: floatArrayType,
//             intArrayType: intArrayType,
//             byteArrayType: byteArrayType,
//             byteString: new TextDecoder().decode(user.byteData)
//         });
//     `)
// 	defer result.Free()

// 	if result.IsException() {
// 		err := ctx.Exception()
// 		fmt.Println("Error:", err.Error())
// 		return
// 	}

// 	fmt.Println("结果:", result.JSONStringify())

// 	// 将 JavaScript 值 unmarshal 回 Go
// 	var userData map[string]interface{}
// 	err = ctx.Unmarshal(jsVal, &userData)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Printf("Unmarshal 结果: %+v\n", userData)
// }

// func ModTest() {
// 	rt := quickjs.NewRuntime()
// 	defer rt.Close()
// 	ctx := rt.NewContext()
// 	defer ctx.Close()

// 	// 创建包含 Go 函数和值的数学模块
// 	addFunc := ctx.NewFunction(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
// 		if len(args) >= 2 {
// 			return ctx.NewFloat64(args[0].ToFloat64() + args[1].ToFloat64())
// 		}
// 		return ctx.NewFloat64(0)
// 	})
// 	defer addFunc.Free()

// 	// 使用流畅 API 构建模块
// 	module := quickjs.NewModuleBuilder("math").
// 		Export("PI", ctx.NewFloat64(3.14159)).
// 		Export("add", addFunc).
// 		Export("version", ctx.NewString("1.0.0")).
// 		Export("default", ctx.NewString("数学模块"))

// 	err := module.Build(ctx)
// 	if err != nil {
// 		panic(err)
// 	}

// 	// 在 JavaScript 中使用标准 ES6 import 使用模块
// 	result := ctx.Eval(`
//         (async function() {
//             // 命名导入
//             const { PI, add, version } = await import('math');

//             // 使用导入的函数和值
//             const sum = add(PI, 1.0);
//             return { sum, version };
//         })()
//     `, quickjs.EvalAwait(true))
// 	defer result.Free()

// 	if result.IsException() {
// 		err := ctx.Exception()
// 		panic(err)
// 	}

// 	fmt.Println("模块结果:", result.JSONStringify())
// 	// 输出: 模块结果: {"sum":4.14159,"version":"1.0.0"}
// }
