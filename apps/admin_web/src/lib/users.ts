"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type {
  AdminUser,
  UserListResponse,
  UserRole,
  UserStatus,
} from "@/lib/types";

export interface UserFilters {
  offset?: number;
  limit?: number;
  role?: UserRole | "ALL";
  status?: UserStatus | "ALL";
  search?: string;
}

export function useUsers(filters: UserFilters) {
  return useQuery({
    queryKey: ["users", filters],
    queryFn: async (): Promise<UserListResponse> => {
      const { data } = await api.get<{ success: boolean; data: UserListResponse }>(
        "/api/v1/admin/users",
        {
          params: {
            ...filters,
            role: filters.role === "ALL" ? undefined : filters.role,
            status: filters.status === "ALL" ? undefined : filters.status,
            search: filters.search || undefined,
          },
        },
      );
      return data.data;
    },
  });
}

export interface UserActionResult {
  user_id: string;
  status: string;
  updated_at: string;
}

export function useUserAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      userId,
      action,
    }: {
      userId: string;
      action: "freeze" | "suspend" | "ban" | "unfreeze";
    }): Promise<UserActionResult> => {
      const { data } = await api.patch<{
        success: boolean;
        data: UserActionResult;
      }>(`/api/v1/admin/users/${encodeURIComponent(userId)}/${action}`);
      return data.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

export function useUserDetail(userId: string | null) {
  return useQuery({
    queryKey: ["user", "detail", userId],
    enabled: !!userId,
    queryFn: async (): Promise<AdminUser> => {
      const { data } = await api.get<{ success: boolean; data: AdminUser }>(
        `/api/v1/admin/users/${encodeURIComponent(userId!)}`,
      );
      return data.data;
    },
  });
}
