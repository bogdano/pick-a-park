# pick-a-park.com
## Quickly pick a US National Park to visit, based on driving distance from your location, and the weather at the park.

A learning exercise in Go programming, this project was inspired by the hours I spent trying to pick a national park for a roadtrip during spring break, while I was studying in the US. In mid-March most of the parks around me were still too cold for a longer camping trip, and so I would spend hours cross-referencing the weather at the parks and trying to pick the closest one that my sleeping bag could handle.

The app was built using the [National Park Service API](https://www.nps.gov/subjects/developer/api-documentation.htm), the [OpenWeatherMap API](https://openweathermap.org/api), and the [Mapbox API](https://www.mapbox.com/).

Most of the **Go** code deals with wrangling the data from the NPS API. Their images are 10MB+ in size, and so one of the primary tasks was to resize and compress them before storing them in my DB, in an efficient way. Other optimizations include storing certain responses in my DB as JSON strings, minimizing the number of queries to the Mapbox API by using the Haversine function to narrow down parks within a certain radius of the coordinates provided by the browser's Geolocation API, and storing certain responses in localstorage.

For templating, I used **templ** ([templ.guide](https://templ.guide)). The type safety and LSP saved me several times during development, and template generation is very quick at Go build time.

I used **HTMX** ([https://htmx.org](https://htmx-org)) to handle state because I am interested in exploring how it feels to use it in real-world projects. Its simplicity is enticing, but I am still trying to narrow down specifically what kinds of usecases it would provide a massive benefit for in terms of reduced complexity (as is the case with this project), and which kinds of usecases it would eventually appear to be a hindrance for. All additional client-side logic was written in pure JS.

I opted to use **Pocketbase** ([https://pocketbase.io](https://pocketbase.io))  for this project because I love the idea of it and wanted to experience working with it. Considering the final scope, it would have made more sense to use something like Turso or basic SQLite distributed via LiteFS (as of now the app doesn't utilize the auth/email functionalities because I didn't feel it would contribute to the usability of the app).

The app is hosted on [fly.io](https://fly.io):

[https://pick-a-park.com](https://pick-a-park.com).


**PS. There is nothing stopping this project from including National Parks from all nations, other than the gargantuan task of accumulating the data necessary (which does not appear to be consolidated anywhere). According to Wikipedia, there are 3,367 national parks worldwide.**

***If anybody has a suggestion as to how to make this happen, please let me know!***
