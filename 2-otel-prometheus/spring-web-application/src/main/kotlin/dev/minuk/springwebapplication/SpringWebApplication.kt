package dev.minuk.springwebapplication

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication
import org.springframework.context.annotation.ComponentScan
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.RestController
import kotlin.random.Random

@SpringBootApplication
class SpringWebApplication

fun main(args: Array<String>) {
    runApplication<SpringWebApplication>(*args)
}

@RestController
class HelloController {
    @GetMapping("/hello")
    fun hello(): String {
        Thread.sleep(Random.nextLong(1000) + 30) // Random delay between 500ms and 1500ms
        return "Hello, World!"
    }
}