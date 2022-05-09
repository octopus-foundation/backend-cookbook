Intelligence itself, computations in general, even the course of the biological evolution is all about creating and controlling increasingly complex processes. And the issue with controlling complex data flows is error handling and processing.

Computation is perfect and arguably can be as fast as one may probably need. Input and output is what rises all the issues in software development. This is what is hard. So each time a new software development paradigm invented it always brings something new to error handling. Evolution of error handling can be tracked starting with assembly language:

```assembly
; x86 assembly program for DOS
;  1. read block from file
;  2. check for errors, if no errors - exit
;  3. output error message "error reading file"
; 4. exit

; read block from file
mov dx, 0h     ; file handle
mov cx, 1024h  ; number of bytes to read
mov ah, 3fh    ; read file function
int 21h

; check for errors, if no errors - exit
cmp ax, 0
jz exit

; output error message "error reading file"
mov dx, offset error_message
mov ah, 09h
int 21h

; exit
exit:
mov ah, 4ch
int 21h
error_message: db 'error reading file', 0dh, 0ah, '$'
```

As seen above CPU designers had to incorporate compare and jump instructions and even optimize values, creating commands like `jz` - `jump if zero`.

Another way, invented in times of C++ is exceptions, a very popular concept, Java software entirely built around exceptions and code which handles it. It's often ok to see such constructs around any input / output related blocks of code:

```java
try {
    // read block from file
} catch (IOException e) {
    // output error message "error reading file"
}
```

The problem with exceptions is a major flow in design. Exceptions are not meant to be used to control execution flow, yet that's exactly what is needed when controlling complex systems. This contradiction leads to all types of issues java architectures are suffering from, and the most important of them is the loss of context. Exceptions are reported not where they were caught and sometimes, basically always if no precautions were specifically designed in software, texts of errors are meaningless. For instance - what one can tell from knowing that `resource is busy` when trying to increase user's balance?

The point is that errors have to be treated as first class citizens in software architecture. They can not be left behind or ignored. They are a major part of data flow control.

Another take was made in academia to handle the issue. Haskell and friends introduced `monads`, namely `Maybe` monad, known in Scala as `Either`:

```scala
import scala.util.{Try, Success, Failure}

val result: Try[Int] = Try(1/0)

result match {
  case Success(v) => println(s"got $v")
  case Failure(e) => println(s"exception $e")
}
```

The idea behind this is to move computations which may fail into monads. The problem is that monads are not composable, which means it's really hard to use them in real life.

Computer scientists invented lots of tricks to compose monads, and some look really nice to work with. One of the best solutions based on similar approach was Erlang, where error handling is done by checking whether values are correct, for instance - a call to open file should return "ok":

```erlang
case file:open(FileName) of
    {ok, Handle} ->
        % do something with Handle
    {error, Reason} ->
        % print error message
end
```

This way, errors are first class citizens and can be handled in a very predictable way. On the downside, the code is verbose and checking values at each step is really annoying.

Complex systems can not be built on try-catch and/or monads or suffer from extensive error checking.

Go language introduced a very different approach to handling errors while maintaining execution flow. Go encourages developers to return errors as second argument of input / output functions and handle errors wherever and whenever it makes sense:

```go
package main

import (
	"fmt"
	"io/ioutil"
)

func main() {
	filename := "file"
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	fmt.Printf("File contents: %s", content)
}
```

As seen above, and this is the only way to work with errors in Go, developers have to handle errors explicitly. This way, errors are first class citizens and can be handled in a very predictable way while maintaining all required context manually.
