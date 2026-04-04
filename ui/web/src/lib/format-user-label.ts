type ContactLike = { display_name?: string; username?: string } | null;
type Resolver = (id: string) => ContactLike;
export type UserLabelOptions = {
  resolve?: Resolver;
  metadata?: Record<string, string>;
};

type UserLabelArg = Resolver | UserLabelOptions | undefined;

function normalizeUserLabelArg(arg?: UserLabelArg): UserLabelOptions {
  if (!arg) return {};
  if (typeof arg === "function") return { resolve: arg };
  return arg;
}

/**
 * Format a user/sender ID into a human-readable label.
 * Display hierarchy:
 *   - For group:* IDs with metadata.chat_title: use the channel title
 *   - display_name from resolver > @username > formatted ID fallback
 */
export function formatUserLabel(userId: string, arg?: UserLabelArg): string {
  if (!userId) return "";

  const { resolve, metadata } = normalizeUserLabelArg(arg);

  // Special: group:* IDs with chat_title metadata get friendly channel name
  if (userId.startsWith("group:") && metadata?.chat_title) {
    return metadata.chat_title;
  }

  // Try contact resolver first
  if (resolve) {
    const contact = resolve(userId);
    if (contact?.display_name) return contact.display_name;
    if (contact?.username) return `@${contact.username}`;
  }

  // Special cases
  if (userId === "system") return "System";
  if (userId.startsWith("group:")) {
    const parts = userId.split(":");
    if (parts.length >= 3) {
      const channel = parts[1]!.charAt(0).toUpperCase() + parts[1]!.slice(1);
      return `${channel} ${parts.slice(2).join(":")}`;
    }
  }

  // Fallback: prefix numeric IDs with #, string IDs with @
  if (/^\d+$/.test(userId)) return `#${userId}`;
  return userId;
}
