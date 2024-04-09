# bencher


bencher benchmarks a url and returns the number of requests sent, together with total bytes sent/received.       
The bytes sent/received is inclusive of request line, request headers, response line, response headers & response body.     
It only currently works with http GET method.     

Usage:
```
git clone git@github.com:komuw/bencher.git
cd bencher/
go build -o bencher .

./bencher -u https://example.com -c 10 # send 10 requests to example..com

Args:
-u string   # URL to send requests to.
-c uint64 # Total number of requests to send.
```

The response looks like.   
```
allErrors: <nil>
totalBenchmarkRequests: 10
totalBenchmarkRequestSuccess: 10
totalBenchmarkRequestFailure: 0
totalBenchmarkRequestHeaderSize: 120 bytes.
totalBenchmarkResponseHeaderSize: 3100 bytes.
totalBenchmarkResponseBodySize: 12560 bytes.
totalBenchmarkThroughput: 15780 bytes. <- includes request/response line,headers,body.
```

**Note:** `bencher` is not very accurate.
