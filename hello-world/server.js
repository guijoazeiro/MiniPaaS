const http = require("http");

const PORT = 8080;

let i = 0;
setInterval(() => {
  i += 1;
  console.log(`heartbeat ${i}`);
  if (i % 3 === 0) console.error(`even beat ${i}`);
}, 1000);

http
  .createServer((req, res) => {
    console.log(`${req.method} ${req.url}`);
    res.writeHead(200);
    res.end("hello\n");
  })
  .listen(PORT, () => console.log(`listening on :${PORT}`));
