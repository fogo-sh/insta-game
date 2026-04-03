const APP_ID = process.env.DISCORD_APP_ID ?? "";
const BOT_TOKEN = process.env.DISCORD_BOT_TOKEN ?? "";

if (!APP_ID || !BOT_TOKEN) {
  console.error("DISCORD_APP_ID and DISCORD_BOT_TOKEN must be set");
  process.exit(1);
}

const GAME_CHOICES = [
  { name: "Xonotic", value: "xonotic" },
  { name: "QSS-M (Quake 1)", value: "qssm" },
  { name: "q2repro (Quake 2)", value: "q2repro" },
  { name: "BZFlag", value: "bzflag" },
];

const commands = [
  {
    name: "start",
    description: "Start a game server",
    options: [{
      name: "game",
      description: "Which game to start",
      type: 3, // STRING
      required: true,
      choices: GAME_CHOICES,
    }],
  },
  {
    name: "stop",
    description: "Stop a game server",
    options: [{
      name: "game",
      description: "Which game to stop",
      type: 3,
      required: true,
      choices: GAME_CHOICES,
    }],
  },
  {
    name: "status",
    description: "Get the status of a game server",
    options: [{
      name: "game",
      description: "Which game to check",
      type: 3,
      required: true,
      choices: GAME_CHOICES,
    }],
  },
];

async function main() {
  const res = await fetch(`https://discord.com/api/v10/applications/${APP_ID}/commands`, {
    method: "PUT",
    headers: {
      "Authorization": `Bot ${BOT_TOKEN}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(commands),
  });

  if (!res.ok) {
    const err = await res.text();
    console.error("Failed to register commands:", err);
    process.exit(1);
  }

  const data = await res.json();
  console.log("Registered commands:", JSON.stringify(data, null, 2));
}

main();
