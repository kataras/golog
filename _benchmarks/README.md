# Running the benchmarks

```bash
$ cd $GOPATH/src/github.com/kataras/golog/_benchmarks
$ go get ./...
$ go test -v -bench=. -benchtime=20s
```

# Results

| test | times ran (large is better) |  ns/op (small is better) | B/op (small is better) | allocs/op (small is better) |
| -----------|--------|-------------|-------------|-------------|
| **BenchmarkGologPrint** | 10000000 | 4032 ns/op | 1082 B/op | 32 allocs/op |
| BenchmarkLogrusPrint | &nbsp; 3000000 | 9421 ns/op | 1611 B/op | 64 allocs/op |

> Feel free to send a [PR](https://github.com/kataras/golog/pulls) of your own loger benchmark to put it here!

<details>
<summary>Details</summary>

```bash
C:\mygopath\src\github.com\kataras\golog\_benchmarks>go test -v -bench=. -benchtime=20s
goos: windows
goarch: amd64
pkg: github.com/kataras/golog/_benchmarks
BenchmarkGologPrint-8           10000000              4032 ns/op            1082 B/op         32 allocs/op
BenchmarkLogrusPrint-8           3000000              9421 ns/op            1611 B/op         64 allocs/op
PASS
ok      github.com/kataras/golog/_benchmarks    82.141s
```

Date: Th 27 July 2017

Processor: Intel(R) Core(TM) i7-4710HQ CPU @ 2.50GHz 2.50Ghz

Ram: 8.00GB
</details>