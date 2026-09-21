"use client";

import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { TransactionDetail, ReverseTransactionResponse } from "@/lib/types";

export function useTransactionDetail(id: string | null) {
  return useQuery({
    queryKey: ["transaction", id],
    enabled: !!id && id.trim().length > 0,
    queryFn: async (): Promise<TransactionDetail> => {
      const { data } = await api.get<{ success: boolean; data: TransactionDetail }>(
        `/api/v1/admin/transactions/${encodeURIComponent(id!)}`,
      );
      return data.data;
    },
  });
}

export function useReverseTransaction() {
  return useMutation({
    mutationFn: async ({
      id,
      reason,
      notes,
    }: {
      id: string;
      reason: string;
      notes: string;
    }): Promise<ReverseTransactionResponse> => {
      const { data } = await api.post<{
        success: boolean;
        data: ReverseTransactionResponse;
      }>(`/api/v1/admin/transactions/${encodeURIComponent(id)}/reverse`, {
        reason,
        notes,
      });
      return data.data;
    },
  });
}
