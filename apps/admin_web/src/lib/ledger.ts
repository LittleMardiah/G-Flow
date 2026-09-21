"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { LedgerFilters, LedgerPage, BalanceVerification } from "@/lib/types";

export function useLedger(filters: LedgerFilters) {
  return useQuery({
    queryKey: ["ledger", filters],
    queryFn: async (): Promise<LedgerPage> => {
      const { data } = await api.get<{ success: boolean; data: LedgerPage }>(
        "/api/v1/admin/ledger",
        {
          params: filters,
        },
      );
      return data.data;
    },
  });
}

export function useBalanceVerification(walletId: string | null) {
  return useQuery({
    queryKey: ["ledger", "verify", walletId],
    enabled: !!walletId && walletId.trim().length > 0,
    queryFn: async (): Promise<BalanceVerification> => {
      const { data } = await api.get<{
        success: boolean;
        data: BalanceVerification;
      }>(`/api/v1/admin/ledger/verify/${encodeURIComponent(walletId!)}`);
      return data.data;
    },
  });
}

export function useLedgerExport() {
  return useMutation({
    mutationFn: async (filters: LedgerFilters) => {
      const { data } = await api.post("/api/v1/admin/ledger/export", filters, {
        responseType: "blob",
      });
      return data as Blob;
    },
  });
}
