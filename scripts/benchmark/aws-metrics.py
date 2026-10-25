"""Fetch credit/cost evidence for explicitly named benchmark instances only."""

import argparse
import datetime
import json
import pathlib
import subprocess

p = argparse.ArgumentParser()
p.add_argument("--data", type=pathlib.Path, required=True)
a = p.parse_args()
queries = []
for path in sorted(a.data.glob("t4g.*/host.json")):
    host = json.loads(path.read_text())
    if ":stack/insta-game-capacity-" not in host["Stack"]:
        raise SystemExit("refusing a non-benchmark stack")
    for metric, stat in [
        ("CPUUtilization", "Average"),
        ("CPUCreditUsage", "Sum"),
        ("CPUCreditBalance", "Average"),
        ("CPUSurplusCreditBalance", "Average"),
        ("CPUSurplusCreditsCharged", "Sum"),
        ("NetworkIn", "Sum"),
        ("NetworkOut", "Sum"),
    ]:
        queries.append(
            {
                "Id": "m" + str(len(queries)),
                "Label": path.parent.name + "/" + metric,
                "MetricStat": {
                    "Metric": {
                        "Namespace": "AWS/EC2",
                        "MetricName": metric,
                        "Dimensions": [
                            {"Name": "InstanceId", "Value": host["InstanceID"]}
                        ],
                    },
                    "Period": 300,
                    "Stat": stat,
                },
                "ReturnData": True,
            }
        )
if not queries:
    raise SystemExit("no benchmark hosts")
request = {
    "MetricDataQueries": queries,
    "StartTime": datetime.datetime.fromtimestamp(
        min(path.stat().st_mtime for path in a.data.glob("t4g.*/host.json")) - 300,
        datetime.timezone.utc,
    ).isoformat(),
    "EndTime": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "MaxDatapoints": 100800,
}
path = a.data / "cloudwatch-request.json"
path.write_text(json.dumps(request))
result = subprocess.check_output(
    [
        "aws",
        "cloudwatch",
        "get-metric-data",
        "--region",
        "ca-central-1",
        "--cli-input-json",
        "file://" + str(path.resolve()),
    ],
    text=True,
)
(a.data / "cloudwatch.json").write_text(result)
print("saved CloudWatch credit and traffic metrics")
