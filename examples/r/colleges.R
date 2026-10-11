# Colleges in MaKaBo as an sf data frame, plotted on their area.
#   Rscript colleges.R            (needs the sf package)
library(sf)

base <- Sys.getenv("BOMBAYCHITRA_URL", "http://localhost:8080")
colleges <- st_read(paste0(base, "/api/v1/places?area=makabo&category=college,university&named=true&limit=500&format=geojson"), quiet = TRUE)
area <- st_read(paste0(base, "/api/v1/areas/makabo?format=geojson"), quiet = TRUE)

print(colleges[, c("name", "category", "locality")])

png("makabo-colleges.png", width = 900, height = 900)
plot(st_geometry(area), col = "#f3efe7", border = "#e8590c", main = "Colleges in MaKaBo")
plot(st_geometry(colleges), add = TRUE, pch = 19, col = "#3E63DD")
dev.off()
