const child_process = require("child_process");

// little wrapper js to lint the go project in ci pipeline

const ret = child_process.execSync("gofmt -d -e -l -s .", {
    encoding:'utf8',
});
if (ret !== "") {
    console.log(ret);
    process.exit(1);
}
