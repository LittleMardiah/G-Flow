export type MaterialIconName =
  | "person"
  | "arrow_forward"
  | "star"
  | "restaurant"
  | "local_shipping"
  | "account_circle"
  | "notifications"
  | "arrow_upward"
  | "add_card"
  | "receipt_long"
  | "two_wheeler"
  | "directions_car"
  | "inventory_2"
  | "check_circle"
  | "moped"
  | "soup_kitchen"
  | "account_balance_wallet"
  | "qr_code_scanner"
  | "sports_motorsports"
  | "storefront"
  | "check"
  | "mail"
  | "call"
  | "apartment"
  | "verified_user"
  | "send"
  | "expand_more"
  | "terminal"
  | "code"
  | "near_me"
  | "location_on";

export function MaterialIcon({
  name,
  className,
  fill,
}: {
  name: MaterialIconName;
  className?: string;
  fill?: boolean;
}) {
  const style = fill
    ? { fontVariationSettings: "'FILL' 1" as const }
    : undefined;
  return (
    <span
      className={`material-symbols-outlined ${className ?? ""}`}
      style={style}
    >
      {name}
    </span>
  );
}