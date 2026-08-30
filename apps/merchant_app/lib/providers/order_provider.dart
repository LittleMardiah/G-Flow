import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/merchant_order.dart';
import 'auth_provider.dart';

class OrdersState {
  final List<MerchantOrder> orders;
  final bool isLoading;
  final String? error;
  final String? filter;

  const OrdersState({this.orders = const [], this.isLoading = false, this.error, this.filter});

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

  List<MerchantOrder> get visibleOrders {
    final f = state.filter;
    if (f == null || f == 'ALL') return state.orders;
    return state.orders.where((o) => o.displayStatus == f).toList();
  }

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