import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/merchant_order.dart';
import 'auth_provider.dart';

class OrdersState {
  final List<MerchantOrder> orders;
  final bool isLoading;
  final String? error;
  final String? filter;

  const OrdersState({this.orders = const [], this.isLoading = false, this.error, this.filter});

  /// Order yang boleh ditampilkan untuk tab filter aktif.
  ///
  /// Backend `GET /merchants/:id/orders` memfilter dengan
  /// `status::text = $2 OR merchant_status::text = $2`
  /// (internal/food/repository.go:912), sehingga order yang di-reject /
  /// dibatalkan merchant (status='CANCELLED', merchant_status='WAITING' —
  /// CHECK constraint merchant_status tidak mengizinkan 'CANCELLED',
  /// migrations/005_food_send_schema.up.sql:318) tetap ikut masuk ke tab
  /// WAITING. Order terminal disaring ulang di sisi client lewat
  /// [MerchantOrder.displayStatus] (fix TD-138: status terminal menang atas
  /// merchant_status).
  List<MerchantOrder> get visibleOrders {
    final f = filter;
    if (f == null || f == 'ALL') return orders;
    return orders.where((o) => o.displayStatus == f).toList();
  }

  OrdersState copyWith({List<MerchantOrder>? orders, bool? isLoading, String? error, String? filter}) {
    return OrdersState(
      orders: orders ?? this.orders,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      filter: filter ?? this.filter,
    );
  }
}

class OrdersNotifier extends StateNotifier<OrdersState> {
  OrdersNotifier(this.ref) : super(const OrdersState());

  final Ref ref;

  List<MerchantOrder> get visibleOrders => state.visibleOrders;

  Future<void> load({String? status}) async {
    final merchantId = await ref.read(storageProvider).read(key: kMerchantIdKey);
    if (merchantId == null || merchantId.isEmpty) {
      state = state.copyWith(error: 'ID merchant belum tersedia. Login ulang / daftar merchant.');
      return;
    }
    state = state.copyWith(isLoading: true, error: null);
    try {
      final orders = await ref.read(orderServiceProvider).fetchOrders(merchantId, status: status == 'ALL' ? null : status);
      state = state.copyWith(orders: orders, isLoading: false, filter: status);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  Future<bool> setStatus(String orderId, String status) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      await ref.read(orderServiceProvider).updateStatus(orderId, status);
      state = state.copyWith(isLoading: false);
      await load(status: state.filter);
      return true;
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
      return false;
    }
  }

  MerchantOrder? orderById(String orderId) {
    for (final o in state.orders) {
      if (o.id == orderId) return o;
    }
    return null;
  }
}

final orderProvider = StateNotifierProvider<OrdersNotifier, OrdersState>((ref) {
  return OrdersNotifier(ref);
});

final orderDetailProvider = FutureProvider.family<MerchantOrder, String>((ref, orderId) async {
  return ref.read(orderServiceProvider).fetchOrder(orderId);
});