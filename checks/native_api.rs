//! Validate live views and the manifest with the actual released launcher types.
use std::io::{BufRead, BufReader};
use swofty_addon::{manifest::AddonManifest, view::View};

fn walk(value: &serde_json::Value, tables: &mut usize, colours: &mut usize) {
    match value {
        serde_json::Value::Object(map) => {
            assert!(!map.contains_key("image") && !map.contains_key("chart"));
            if map.contains_key("table") { *tables += 1; }
            if map.contains_key("colour") { *colours += 1; }
            for child in map.values() { walk(child, tables, colours); }
        }
        serde_json::Value::Array(values) => for child in values { walk(child, tables, colours); },
        _ => {}
    }
}
fn main() {
    let args: Vec<_> = std::env::args().skip(1).collect();
    let mut views=0; let mut tables=0; let mut colours=0;
    for line in BufReader::new(std::fs::File::open(&args[0]).expect("views file")).lines() {
        let value: serde_json::Value = serde_json::from_str(&line.expect("view line")).expect("view JSON");
        let _: View = serde_json::from_value(value.clone()).expect("released launcher view schema");
        walk(&value, &mut tables, &mut colours); views+=1;
    }
    let manifest=AddonManifest::load(std::path::Path::new(&args[1])).expect("released manifest schema");
    manifest.validate().expect("valid native add-on manifest");
    assert!(views>=2 && tables>=3 && colours>0);
    println!("PASS: {views} live views, {tables} native tables and {colours} colour controls; valid native manifest");
}
