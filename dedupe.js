const fs = require("fs");
const path = require("path");

const filePath = path.join(__dirname, "pkg", "app", "assets", "news-hosts.txt");

// Read the file
fs.readFile(filePath, "utf8", (err, data) => {
  if (err) {
    console.error("Error reading the file:", err);
    return;
  }

  // Process the file
  const uniqueSortedLines = Array.from(
    new Set(
      data
        .split("\n")
        .map((line) => line.trim())
        .filter((line) => line !== "")
    )
  ).sort();

  // Write back to the file
  fs.writeFile(filePath, uniqueSortedLines.join("\n"), "utf8", (writeErr) => {
    if (writeErr) {
      console.error("Error writing to the file:", writeErr);
    } else {
      console.log("File successfully updated!");
    }
  });
});
