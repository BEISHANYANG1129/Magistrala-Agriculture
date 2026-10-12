"""Authorized test5 extension. Preserve old logic and restrict the new branch to our device."""
import argparse, json, pathlib, urllib.request
from platform_inspect import login

WORKSPACE = "cccd00c1-a12c-4d6e-9aa9-d2c52906f2e5"
CHANNEL = "3e692d54-5ccc-4a83-9cfd-ac0080263327"
RULE = "29df7dfe-adee-41b3-865e-4ab7e7b9f000"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--env-file", required=True)
    parser.add_argument("--restore", action="store_true")
    parser.add_argument("--include-south", action="store_true")
    args = parser.parse_args()
    runtime = pathlib.Path(__file__).resolve().parents[1] / "runtime"
    before = json.loads((runtime / "test5-before.json").read_text(encoding="utf-8"))
    state = json.loads((runtime / "test3-session.json").read_text(encoding="utf-8"))
    assert before["workspace"] == state["workspace_id"] == WORKSPACE
    assert before["input_channel"] == state["channel_id"] == CHANNEL
    assert before["id"] == RULE
    token = login(args.env_file)
    url = f"http://localhost:9008/{WORKSPACE}/rules/{RULE}"

    def call(data=None):
        headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
        req = urllib.request.Request(url, json.dumps(data).encode() if data else None,
                                     headers, method="PATCH" if data else "GET")
        with urllib.request.urlopen(req, timeout=15) as response:
            return json.load(response)

    mutable = ("name", "metadata", "input_channel", "input_topic", "logic", "outputs", "schedule", "status")
    current = call()
    expected_path=runtime / "test5-after.json"
    expected = json.loads(expected_path.read_text(encoding="utf-8")) if expected_path.exists() else before
    if any(current.get(k) != expected.get(k) for k in mutable):
        raise RuntimeError("Rule changed since snapshot; refusing to overwrite another edit")
    logic = dict(before["logic"])
    if not args.restore:
        publishers=[state['device_id']]
        if args.include_south:
            south=json.loads((runtime / "test3-south-session.json").read_text(encoding="utf-8"))
            assert south['workspace_id']==WORKSPACE and south['channel_id']==CHANNEL
            publishers.append(south['device_id'])
            backup=runtime / "test5-before-south.json"
            if not backup.exists():backup.write_text(json.dumps(current,ensure_ascii=False,indent=2),encoding='utf-8')
        # Lua JSON-quoted UUID is a valid Lua string literal. Original program stays intact.
        logic["value"] = (
            "-- LLM Go synthetic telemetry: only the dedicated test publisher.\n"
            "if " + " or ".join("message.publisher == "+json.dumps(p) for p in publishers) + " then\n"
            "  return message.payload\nend\n\n" + before["logic"]["value"]
        )
    call({"input_channel": before["input_channel"], "input_topic": before["input_topic"], "logic": logic})
    after = call()
    if after["logic"] != logic or any(after.get(k) != before.get(k) for k in mutable if k != "logic"):
        raise RuntimeError("Rule verification mismatch; inspect saved backup before any further writes")
    destination = "test5-restored.json" if args.restore else "test5-after.json"
    (runtime / destination).write_text(json.dumps(after, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"rule_id": RULE, "restored": args.restore, "original_fields_preserved": True,
                      "test_publisher": state["device_id"]}))


if __name__ == "__main__":
    main()
