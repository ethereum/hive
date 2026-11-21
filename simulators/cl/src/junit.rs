//! Minimal JUnit XML parser. Returns aggregate pass/fail/skip counts plus
//! the list of failed testcase names — enough to surface in the hive test
//! `details` text. The full XML stays retrievable from the client
//! container's HTTP server; we do not transform it into a richer
//! per-testcase hive shape in this PR.

use quick_xml::events::Event;
use quick_xml::Reader;
use std::fmt::Write as _;

#[derive(Debug, Default)]
pub struct JunitSummary {
    pub testsuites: Vec<TestSuiteSummary>,
}

#[derive(Debug, Default)]
pub struct TestSuiteSummary {
    pub name: String,
    pub tests: u32,
    pub failures: u32,
    pub errors: u32,
    pub skipped: u32,
    pub failed_cases: Vec<String>,
}

impl JunitSummary {
    pub fn total_tests(&self) -> u32 {
        self.testsuites.iter().map(|s| s.tests).sum()
    }

    pub fn total_failures(&self) -> u32 {
        self.testsuites.iter().map(|s| s.failures + s.errors).sum()
    }

    pub fn total_skipped(&self) -> u32 {
        self.testsuites.iter().map(|s| s.skipped).sum()
    }

    pub fn is_clean(&self) -> bool {
        self.total_failures() == 0 && !self.testsuites.is_empty()
    }

    pub fn render_details(&self, header: &str, max_failed_to_list: usize) -> String {
        let mut out = String::new();
        let _ = writeln!(
            out,
            "{header}: tests={} failures={} skipped={}",
            self.total_tests(),
            self.total_failures(),
            self.total_skipped(),
        );
        for s in &self.testsuites {
            let _ = writeln!(
                out,
                "  - {} : tests={} failures={} errors={} skipped={}",
                s.name, s.tests, s.failures, s.errors, s.skipped,
            );
        }
        let mut printed = 0usize;
        for s in &self.testsuites {
            for case in &s.failed_cases {
                if printed >= max_failed_to_list {
                    let _ = writeln!(out, "  ... (more failed cases truncated)");
                    return out;
                }
                let _ = writeln!(out, "  FAIL: {} :: {}", s.name, case);
                printed += 1;
            }
        }
        out
    }
}

/// Parse a JUnit XML document. Best-effort: returns a synthetic parse-error
/// testsuite on XML errors so the caller can surface the malformed file in
/// the hive details text instead of silently treating it as zero tests.
pub fn parse(xml: &str) -> JunitSummary {
    let mut reader = Reader::from_str(xml);
    reader.config_mut().trim_text(true);

    let mut summary = JunitSummary::default();
    let mut current_suite: Option<TestSuiteSummary> = None;
    let mut current_case_name: Option<String> = None;
    let mut current_case_failed = false;
    let mut buf = Vec::new();

    loop {
        match reader.read_event_into(&mut buf) {
            Ok(Event::Start(e)) => match e.name().as_ref() {
                b"testsuite" => {
                    current_suite = Some(parse_testsuite_attrs(&e));
                }
                b"testcase" => {
                    current_case_name = Some(parse_testcase_name(&e));
                    current_case_failed = false;
                }
                b"failure" | b"error" => {
                    current_case_failed = true;
                }
                _ => {}
            },
            Ok(Event::Empty(e)) => match e.name().as_ref() {
                b"testsuite" => {
                    summary.testsuites.push(parse_testsuite_attrs(&e));
                }
                b"testcase" => {
                    // Self-closing testcase: a pass with no nested failure/error.
                    // The suite-level `tests` attribute already counts it.
                }
                b"failure" | b"error" => {
                    if let (Some(suite), Some(name)) =
                        (current_suite.as_mut(), current_case_name.clone())
                    {
                        suite.failed_cases.push(name);
                    }
                    current_case_failed = true;
                }
                _ => {}
            },
            Ok(Event::End(e)) => match e.name().as_ref() {
                b"testsuite" => {
                    if let Some(s) = current_suite.take() {
                        summary.testsuites.push(s);
                    }
                }
                b"testcase" => {
                    if current_case_failed {
                        if let (Some(suite), Some(name)) =
                            (current_suite.as_mut(), current_case_name.take())
                        {
                            suite.failed_cases.push(name);
                        }
                    }
                    current_case_name = None;
                    current_case_failed = false;
                }
                _ => {}
            },
            Ok(Event::Eof) => break,
            Err(err) => {
                summary.testsuites.push(TestSuiteSummary {
                    name: "<parse-error>".to_string(),
                    tests: 1,
                    failures: 1,
                    failed_cases: vec![format!("XML parse error: {err}")],
                    ..Default::default()
                });
                break;
            }
            _ => {}
        }
        buf.clear();
    }

    summary
}

fn parse_testsuite_attrs(e: &quick_xml::events::BytesStart) -> TestSuiteSummary {
    let mut s = TestSuiteSummary::default();
    for attr in e.attributes().flatten() {
        let val = String::from_utf8_lossy(&attr.value).into_owned();
        match attr.key.as_ref() {
            b"name" => s.name = val,
            b"tests" => s.tests = val.parse().unwrap_or(0),
            b"failures" => s.failures = val.parse().unwrap_or(0),
            b"errors" => s.errors = val.parse().unwrap_or(0),
            b"skipped" => s.skipped = val.parse().unwrap_or(0),
            _ => {}
        }
    }
    s
}

fn parse_testcase_name(e: &quick_xml::events::BytesStart) -> String {
    let mut classname = String::new();
    let mut name = String::new();
    for attr in e.attributes().flatten() {
        let val = String::from_utf8_lossy(&attr.value).into_owned();
        match attr.key.as_ref() {
            b"classname" => classname = val,
            b"name" => name = val,
            _ => {}
        }
    }
    if classname.is_empty() {
        name
    } else {
        format!("{classname}.{name}")
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_passing_suite() {
        let xml = r#"<?xml version="1.0"?>
            <testsuite name="bls" tests="3" failures="0" errors="0" skipped="0">
                <testcase name="sign"/>
                <testcase name="verify"/>
                <testcase name="aggregate"/>
            </testsuite>"#;
        let s = parse(xml);
        assert_eq!(s.testsuites.len(), 1);
        assert_eq!(s.total_tests(), 3);
        assert_eq!(s.total_failures(), 0);
        assert!(s.is_clean());
    }

    #[test]
    fn parses_failing_case_with_nested_failure() {
        let xml = r#"<?xml version="1.0"?>
            <testsuite name="sanity" tests="2" failures="1" errors="0" skipped="0">
                <testcase name="pass-case"/>
                <testcase name="fail-case">
                    <failure message="state root mismatch">expected 0xaa, got 0xbb</failure>
                </testcase>
            </testsuite>"#;
        let s = parse(xml);
        assert_eq!(s.total_failures(), 1);
        assert_eq!(s.testsuites[0].failed_cases, vec!["fail-case".to_string()]);
    }

    #[test]
    fn handles_multiple_testsuites() {
        let xml = r#"<?xml version="1.0"?>
            <testsuites>
                <testsuite name="bls" tests="1" failures="0"/>
                <testsuite name="sanity" tests="2" failures="1"/>
            </testsuites>"#;
        let s = parse(xml);
        assert_eq!(s.testsuites.len(), 2);
        assert_eq!(s.total_tests(), 3);
        assert_eq!(s.total_failures(), 1);
    }
}
