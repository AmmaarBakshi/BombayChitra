//! Prints statistics for an area.
//!     cargo run -- makabo
use std::env;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let base = env::var("BOMBAYCHITRA_URL").unwrap_or_else(|_| "http://localhost:8080".into());
    let area = env::args().nth(1).unwrap_or_else(|| "makabo".into());

    let stats: serde_json::Value =
        reqwest::blocking::get(format!("{base}/api/v1/stats?area={area}"))?.error_for_status()?.json()?;

    let totals = &stats["totals"];
    println!("Area: {area}");
    for key in ["places", "buildings", "building_avg_height_m", "building_max_height_m", "streets", "bridges", "bridges_km"] {
        println!("  {key:<24} {}", totals[key]);
    }

    let mut top: Vec<(String, f64)> = stats["places_by_category"]
        .as_object()
        .map(|m| m.iter().map(|(k, v)| (k.clone(), v.as_f64().unwrap_or(0.0))).collect())
        .unwrap_or_default();
    top.sort_by(|a, b| b.1.total_cmp(&a.1));
    println!("Most common places:");
    for (category, count) in top.iter().take(8) {
        println!("  {category:<24} {count}");
    }
    Ok(())
}
