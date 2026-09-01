"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type {
  DashboardKPIs,
  TransactionListResponse,
} from "@/lib/types";

export function useDashboardKPIs() {
  return useQuery({
    queryKey: ["dashboard", "kpis"],
    queryFn: async (): Promise<DashboardKPIs> => {
      const { data } = await api.get<DashboardKPIs>("/admin/dashboard/kpis");
      return data;
    },
  });
}

export function useLatestTransactions(limit = 10) {
  return useQuery({
    queryKey: ["dashboard", "transactions", limit],
    queryFn: async (): Promise<TransactionListResponse> => {
      const { data } = await api.get<TransactionListResponse>(
        "/admin/dashboard/transactions",
        { params: { limit } },
      );
      return data;
    },
  });
}
