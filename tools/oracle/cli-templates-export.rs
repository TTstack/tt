
    pub fn fixtures() -> serde_json::Value {
        let mut cases = Vec::new();
        for service in ["tt-agent", "tt-ctl"] {
            for authenticated in [false, true] {
                let command = format!("/opt/tt/bin/{service} --listen 0.0.0.0:9100");
                let path = format!("/opt/tt/etc/{service}.env");
                let env = authenticated.then_some(path.as_str());
                cases.push(serde_json::json!({
                    "service": service, "authenticated": authenticated,
                    "unit": systemd_unit(service, &command, service == "tt-agent", env),
                    "openrc": openrc_initd(service, &command),
                    "remote": remote_setup_script(service, &command, "fixture", "/home/fixture", "/opt/tt", "/tmp/ttstack-deploy.fixture", &[service], &["/srv/images"], "testhost", env.map(|path| (path, "TT_API_KEY=fixture-key\n")))
                }));
            }
        }
        serde_json::json!({"revision":"1ad83a9feed5fac820d74b5e7ef752e50a552740", "cases":cases})
    }
