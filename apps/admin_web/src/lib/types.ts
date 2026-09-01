export interface DashboardKPIs {
  active_orders: number;
  total_transaction_volume: number;
  avg_fare: number;
  revenue_today: number;
  error_rate: number;
  order_status_breakdown: Array<{
    status: string;
    count: number;
  }>;
}

export interface OrderStatusSlice {
  status: string;
  count: number;
}

export interface Transaction {
  id: string;
  wallet_id: string;
  entry_type: string;
  amount: number;
  reference: string;
  created_at: string;
  status?: string;
  note?: string;
}

export interface TransactionListResponse {
  total_count: number;
  transactions: Transaction[];
}

export interface LedgerFilters {
  offset?: number;
  limit?: number;
  date_from?: string;
  date_to?: string;
  wallet_type?: string;
  entry_type?: string;
  search?: string;
}

export interface LedgerPage {
  ledger: Transaction[];
  total_count: number;
}

export interface BalanceVerification {
  wallet_id: string;
  total_debit: number;
  total_credit: number;
  discrepancy: number;
  status: "BALANCED" | "MISMATCH";
}

export type UserRole = "CUSTOMER" | "DRIVER" | "MERCHANT" | "ADMIN";
export type UserStatus = "ACTIVE" | "FROZEN" | "SUSPENDED" | "BANNED";

export interface AdminUser {
  id: string;
  email: string;
  phone?: string;
  role: UserRole;
  status: UserStatus;
  balance: number;
  created_at: string;
}

export interface UserListResponse {
  total_count: number;
  users: AdminUser[];
}
