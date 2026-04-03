import { verifyKey, InteractionType, InteractionResponseType } from "discord-interactions";
import type { Context } from "hono";
import type { Backend } from "./backend.js";

const DISCORD_PUBLIC_KEY = process.env.DISCORD_PUBLIC_KEY ?? "";
const DISCORD_BOT_TOKEN = process.env.DISCORD_BOT_TOKEN ?? "";
const DISCORD_APP_ID = process.env.DISCORD_APP_ID ?? "";

export function makeDiscordHandler(backend: Backend) {
  return (c: Context) => discordHandler(c, backend);
}

async function discordHandler(c: Context, backend: Backend): Promise<Response> {
  const signature = c.req.header("x-signature-ed25519") ?? "";
  const timestamp = c.req.header("x-signature-timestamp") ?? "";
  const rawBody = await c.req.text();

  const isValid = await verifyKey(rawBody, signature, timestamp, DISCORD_PUBLIC_KEY);
  if (!isValid) return c.text("invalid signature", 401);

  const interaction = JSON.parse(rawBody) as {
    type: number;
    id: string;
    token: string;
    data?: { name: string; options?: Array<{ name: string; value: string }> };
  };

  // PING
  if (interaction.type === InteractionType.PING) {
    return c.json({ type: InteractionResponseType.PONG });
  }

  // Slash command
  if (interaction.type === InteractionType.APPLICATION_COMMAND) {
    const name = interaction.data?.name ?? "";
    const gameName = interaction.data?.options?.find(o => o.name === "game")?.value ?? "";
    const games = backend.getGames();
    const config = games[gameName];

    if (!config) {
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: `Unknown game: \`${gameName}\`` },
      });
    }

    if (name === "status") {
      const state = await backend.getGameState(config);
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) },
      });
    }

    if (name === "stop") {
      const state = await backend.stopGame(config);
      return c.json({
        type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) },
      });
    }

    if (name === "start") {
      // Defer immediately — start can take up to ~50s
      void sendFollowup(interaction.token, gameName, config, backend);
      return c.json({ type: InteractionResponseType.DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE });
    }

    return c.json({
      type: InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
      data: { content: `Unknown command: \`${name}\`` },
    });
  }

  return c.text("unhandled interaction type", 400);
}

async function sendFollowup(
  interactionToken: string,
  gameName: string,
  config: Parameters<Backend["startGame"]>[0],
  backend: Backend,
): Promise<void> {
  const state = await backend.startGame(config);
  const content = formatState(gameName, state);

  await fetch(
    `https://discord.com/api/v10/webhooks/${DISCORD_APP_ID}/${interactionToken}/messages/@original`,
    {
      method: "PATCH",
      headers: {
        "Authorization": `Bot ${DISCORD_BOT_TOKEN}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ content }),
    }
  );
}

function formatState(gameName: string, state: { status: string; publicIp?: string; players: number }): string {
  const parts = [`**${gameName}** — ${state.status}`];
  if (state.publicIp) parts.push(`\`${state.publicIp}\``);
  if (state.players > 0) parts.push(`${state.players} player(s)`);
  return parts.join(" — ");
}
