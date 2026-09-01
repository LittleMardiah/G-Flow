"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useUsers, useUserAction, useUserDetail, type UserFilters } from "@/lib/users";
import type { AdminUser, UserRole, UserStatus } from "@/lib/types";

const PAGE_SIZE = 50;

function statusVariant(status: UserStatus): "green" | "amber" | "red" | "gray" {
  switch (status) {
    case "ACTIVE":
      return "green";
    case "FROZEN":
      return "amber";
    case "SUSPENDED":
      return "amber";
    case "BANNED":
      return "red";
    default:
      return "gray";
  }
}

export function UsersClient() {
  const [filters, setFilters] = useState<UserFilters>({
    offset: 0,
    limit: PAGE_SIZE,
    role: "ALL",
    status: "ALL",
    search: "",
  });
  const [selected, setSelected] = useState<AdminUser | null>(null);

  const users = useUsers(filters);
  const action = useUserAction();
  const detail = useUserDetail(selected ? selected.id : null);
  const detailUser = detail.data ?? selected;

  const update = (patch: Partial<UserFilters>) => {
    setFilters((prev) => ({ ...prev, ...patch, offset: 0 }));
  };

  const performAction = (actionName: "freeze" | "suspend" | "ban" | "unfreeze") => {
    if (!selected) return;
    action.mutate(
      { userId: selected.id, action: actionName },
      {
        onSuccess: () => {
          setSelected((prev) =>
            prev ? { ...prev, status: statusFromAction(actionName) } : prev,
          );
        },
      },
    );
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3 lg:grid-cols-5">
            <Select
              value={filters.role}
              onChange={(e) => update({ role: e.target.value as UserRole | "ALL" })}
            >
              <option value="ALL">All Roles</option>
              <option value="CUSTOMER">CUSTOMER</option>
              <option value="DRIVER">DRIVER</option>
              <option value="MERCHANT">MERCHANT</option>
              <option value="ADMIN">ADMIN</option>
            </Select>
            <Select
              value={filters.status}
              onChange={(e) =>
                update({ status: e.target.value as UserStatus | "ALL" })
              }
            >
              <option value="ALL">All Statuses</option>
              <option value="ACTIVE">ACTIVE</option>
              <option value="FROZEN">FROZEN</option>
              <option value="SUSPENDED">SUSPENDED</option>
              <option value="BANNED">BANNED</option>
            </Select>
            <Input
              value={filters.search}
              onChange={(e) =>
                setFilters((prev) => ({ ...prev, search: e.target.value }))
              }
              onKeyDown={(e) =>
                e.key === "Enter" &&
                setFilters((prev) => ({ ...prev, offset: 0 }))
              }
              placeholder="Search email"
            />
            <Button
              variant="outline"
              onClick={() => setFilters((prev) => ({ ...prev, offset: 0 }))}
            >
              Search
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Users</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User ID</TableHead>
                <TableHead>Email</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Balance</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {(users.data?.users ?? []).map((user) => (
                <TableRow key={user.id}>
                  <TableCell className="font-mono text-xs">
                    {user.id.slice(0, 8)}
                  </TableCell>
                  <TableCell>{user.email}</TableCell>
                  <TableCell>
                    <Badge variant="blue">{user.role}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(user.status)}>
                      {user.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-medium">
                    {formatAmount(user.balance)}
                  </TableCell>
                  <TableCell>
                    <Button variant="outline" onClick={() => setSelected(user)}>
                      Detail
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
              {(users.data?.users ?? []).length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="py-6 text-center text-gray-400">
                    No users found
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-between">
            <button
              className="text-sm text-blue-600 disabled:text-gray-300"
              disabled={(filters.offset ?? 0) === 0}
              onClick={() =>
                setFilters((prev) => ({
                  ...prev,
                  offset: Math.max(0, (prev.offset ?? 0) - PAGE_SIZE),
                }))
              }
            >
              Previous
            </button>
            <span className="text-sm text-gray-500">
              {users.data?.total_count ?? 0} total
            </span>
            <button
              className="text-sm text-blue-600 disabled:text-gray-300"
              disabled={(users.data?.users ?? []).length < (filters.limit ?? PAGE_SIZE)}
              onClick={() =>
                setFilters((prev) => ({
                  ...prev,
                  offset: (prev.offset ?? 0) + PAGE_SIZE,
                }))
              }
            >
              Next
            </button>
          </div>
        </CardContent>
      </Card>

      {selected && detailUser && (
        <UserDetailModal
          user={detailUser}
          loading={detail.isLoading}
          actionPending={action.isPending}
          onClose={() => setSelected(null)}
          onAction={performAction}
        />
      )}
    </div>
  );
}

function UserDetailModal({
  user,
  loading,
  actionPending,
  onClose,
  onAction,
}: {
  user: AdminUser;
  loading: boolean;
  actionPending: boolean;
  onClose: () => void;
  onAction: (a: "freeze" | "suspend" | "ban" | "unfreeze") => void;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-lg rounded-xl bg-white p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">User Detail</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            ✕
          </button>
        </div>
        {loading ? (
          <p className="mt-4 text-sm text-gray-400">Loading…</p>
        ) : (
          <>
            <div className="mt-4 space-y-2 text-sm">
              <p>
                <span className="text-gray-500">ID:</span>{" "}
                <span className="font-mono">{user.id}</span>
              </p>
              <p>
                <span className="text-gray-500">Email:</span> {user.email}
              </p>
              <p>
                <span className="text-gray-500">Phone:</span> {user.phone ?? "-"}
              </p>
              <p>
                <span className="text-gray-500">Role:</span> {user.role}
              </p>
              <p>
                <span className="text-gray-500">Status:</span>{" "}
                <Badge variant={statusVariant(user.status)}>{user.status}</Badge>
              </p>
              <p>
                <span className="text-gray-500">Balance:</span>{" "}
                {formatAmount(user.balance)}
              </p>
              <p>
                <span className="text-gray-500">Joined:</span>{" "}
                {new Date(user.created_at).toLocaleDateString()}
              </p>
            </div>
            <div className="mt-5 flex flex-wrap gap-2">
              <Button
                variant="outline"
                disabled={actionPending}
                onClick={() => onAction("freeze")}
              >
                Freeze
              </Button>
              <Button
                variant="outline"
                disabled={actionPending}
                onClick={() => onAction("suspend")}
              >
                Suspend
              </Button>
              <Button
                variant="danger"
                disabled={actionPending}
                onClick={() => onAction("ban")}
              >
                Ban
              </Button>
              <Button
                variant="ghost"
                disabled={actionPending}
                onClick={() => onAction("unfreeze")}
              >
                Unfreeze
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function formatAmount(value: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value ?? 0);
}

function statusFromAction(
  a: "freeze" | "suspend" | "ban" | "unfreeze",
): UserStatus {
  switch (a) {
    case "freeze":
      return "FROZEN";
    case "suspend":
      return "SUSPENDED";
    case "ban":
      return "BANNED";
    case "unfreeze":
      return "ACTIVE";
  }
}
