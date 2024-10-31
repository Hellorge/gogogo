steps takes for this project:

1. installed go

2. some more installs:
	- go get github.com/tdewolff/minify/v2
	- go get github.com/tdewolff/minify/v2/css
	- go get github.com/tdewolff/minify/v2/js
	- go get github.com/tdewolff/minify/v2/html
	- go get github.com/fsnotify/fsnotify
	- go get github.com/evanw/esbuild
	- and etc...

3. commands:
	cd into cmd
	start server = go run main.go
		- '-prod' for production mode
		- '-spa' for SPA mode
		- '-metrics' for logging metrics data

	build assets = go run build.go
		- '-watch' for file watcher to build assets on change
		- '-concurrency x' to change parallel building to no X
		- use .buildignore file to ignore patters and files in build process

	check metrics = go run stats.go

4. features:
	- SPA and Traditional modes
	- Cache aware request coalescing
	- Production and Development modes
	- Pre compile, minified file serving for production
	- Dynamic cache size based on available memory
	- Template caching
	- Static file serving
	- Thread safety in caching
	- Variable cache life policy
	- Benchmarks for native metrics overhead, test cases for high loads, high traffic, sustained high load, burst traffic
	-

+++
- binary production files, and templating comtenting in build process
- logger and profiler features
- worker pool for request serving
- Consider making it optional or implementing a more efficient, lock-free metrics collection mechanism.
- multi-level cache (memory -> shared memory -> disk)
- in server folder add config file for template selection, header imports, seo and content path aliases
- handle multiple sites using '/server/{sitename}/home'
- client side page caching based on client system specs
- when building remove any cache corresponding to files that are built
- client side background thread asset loading
- nested templates, partial templates
- LFU, Predictive-Gradual cache warming
- Single-source Distributed serving capabilities
- Js MayNeed dynamic module loading system, for known resource combinations, we can use HTTP/2 Server Push to send critical assets before the client requests them.
<link rel="preload" href="/api/content/next-likely-page" as="fetch">
-Implement response compression (e.g., gzip) for textual content. maybe integrated in build process to compress data once and skip runtime load

- implement a testing framework for both client and server
- add built-in support for logging and error tracking
- implement automatic image optimization
- add support for service workers and Progressive Web App (PWA) features

- Client-side enhancements:
	Implement a state management solution (similar to Redux or MobX)
	Add client-side routing with support for nested routes

- Code splitting and lazy loading:
	Implement dynamic imports for JavaScript modules
	Create a system for lazy-loading page components

- Server-side rendering (SSR):
	Implement initial SSR for faster first contentful paint
	Add support for SSR of dynamic routes

- API development features:
	Create a robust routing system with support for RESTful and GraphQL APIs
	Implement middleware support for request/response processing
	Add built-in support for database ORM (like GORM)

- Developer experience:
	Implement hot module replacement for faster development
	Create a CLI tool for scaffolding new projects and components
	Add support for TypeScript on the client-side

- Security enhancements:
	Implement CSRF protection
	Add built-in rate limiting
	Provide easy-to-use authentication and authorization modules

- Internationalization (i18n) and localization (l10n):
	Add support for multi-language content
	Implement locale-based formatting for dates, numbers, etc.

- Documentation and examples:
	Create comprehensive documentation
	Develop a set of example projects showcasing various use cases


- Asynchronous file reading:
For large files or when serving multiple requests simultaneously, we could implement asynchronous file reading. This would allow the server to handle other requests while waiting for file I/O operations to complete.

- Request middleware:
Implementing a middleware system would allow for more flexible request processing, such as logging, authentication, or custom headers. This could be especially useful for API endpoints.

- Enhanced SPA support:
We could expand the SPA support by implementing features like client-side routing helpers, state management integration, or server-side rendering for initial SPA loads to improve SEO and initial load times.

- Improved error handling and logging:
Implementing a centralized error handling and logging system would make it easier to debug issues and monitor the application's health in production.

- Performance profiling:
Adding built-in performance profiling tools would help identify bottlenecks and optimize the framework's performance over time.

- Testing suite:
Developing a comprehensive testing suite, including unit tests and integration tests, would ensure the framework's reliability and make it easier to contribute to the project.

1. Pre-loading content:
We could pre-load frequently accessed content into memory at startup, reducing disk I/O during requests.

3. Optimize template parsing:
Parse templates once at startup and store them in memory.

6. Implement aggressive caching headers:
Set appropriate caching headers to allow client-side caching of static assets.

7. Use a CDN for static assets:
Offload static asset serving to a Content Delivery Network to reduce server load and improve global performance.

8. Implement connection pooling for database connections:
If you're using a database, ensure you're using connection pooling to reduce the overhead of creating new connections.

9. Profile and optimize hot code paths:
Use Go's built-in profiling tools to identify and optimize the most frequently executed code paths.

5. Scalability:
   - While the current design is efficient for a single server, consider how the application might scale horizontally. This could involve distributed caching or load balancing considerations.

9. Metrics and Monitoring:
   - While there's a good start with metrics, consider integrating with standard monitoring solutions (e.g., Prometheus) for better observability in production environments.

### self managed worker pool

### Implement a multi-level cache (memory, disk, distributed) for even faster access.
Add cache warming strategies for frequently accessed content.

### 2. Caching (cache.go)
- The current implementation uses a fixed number of shards (256). Consider making this configurable based on expected load.

## Suggestions for Further Optimization
Integrate distributed tracing (e.g., OpenTelemetry) for better insights into system performance.
Implement real-time alerting for performance anomalies.

1. **Profiling**: Implement more comprehensive profiling, especially in production mode, to identify actual bottlenecks under real-world load.

2. **Connection Pooling**: If the application interacts with databases or other services, ensure proper connection pooling is implemented.

3. **Static Asset Serving**: Consider offloading static asset serving to a dedicated static file server or CDN in production.

5. **Memory Management**: Implement more granular memory management, possibly using sync.Pool for frequently allocated and deallocated objects.

6. **Benchmarking**: Develop a comprehensive suite of benchmarks to measure the impact of any changes on performance.

7. **Hot Path Optimization**: Identify the most frequently executed code paths and focus optimization efforts there.

8. **Async Processing**: For non-critical operations, consider implementing asynchronous processing using message queues or worker pools.
