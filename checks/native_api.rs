//! CI-only validator, compiled inside a pinned upstream launcher checkout.
use std::io::{BufRead, BufReader};
use swofty_addon::view::{Chart, View};

fn walk(value: &serde_json::Value, count: &mut usize) -> Result<(), String> {
    match value {
        serde_json::Value::Object(map) => {
            if map.contains_key("image") {
                return Err("image control emitted by native telemetry".into());
            }
            if let Some(value) = map.get("chart") {
                let chart: Chart = serde_json::from_value(value.clone()).map_err(|e| e.to_string())?;
                chart.validate().map_err(|e| format!("{}: {e}", chart.id))?;
                *count += 1;
            }
            for child in map.values() { walk(child, count)?; }
        }
        serde_json::Value::Array(values) => for child in values { walk(child, count)?; },
        _ => {}
    }
    Ok(())
}

fn main() {
    let mut views = 0;
    let mut charts = 0;
    for path in std::env::args().skip(1) {
        for line in BufReader::new(std::fs::File::open(path).expect("views file")).lines() {
            let value: serde_json::Value = serde_json::from_str(&line.expect("read view")).expect("view JSON");
            let _: View = serde_json::from_value(value.clone()).expect("launcher view schema");
            walk(&value, &mut charts).expect("native chart contract");
            views += 1;
        }
    }
    assert!(views > 0 && charts > 0, "No native chart views validated");
    println!("PASS: {views} complete views and {charts} charts accepted by the pinned launcher API; no image controls");
}
