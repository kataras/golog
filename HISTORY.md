## Sa 29 July 2017 | v0.0.4

Fix an issue occured by previous chnages, which [pio](https://github.com/kataras/pio) appends a trailing new line.

Add a new method `golog#NewLine` which can override the default line breaker chars "\n".


## Th 27 July 2017 | v0.0.3

Increase the logger's performance by reducing the use of buffers on the [pio library](https://github.com/kataras/pio)