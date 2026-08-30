import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../config/constants.dart';
import '../models/driver_order.dart';
import '../models/driver_stop.dart';
import '../services/order_service.dart';
import 'auth_provider.dart';

class OrdersListState {
  const OrdersListState({
    this.orders = const [],
    this.isLoading = false,
    this.error,
    this.message,
  });

  final List<DriverOrder> orders;
  final bool isLoading;
  final String? error;
  final String? message;

  int get activeCount => orders.length;
  bool get isAtCapacity => activeCount >= kMaxActiveOrders;

  OrdersListState copyWith({
    List<DriverOrder>? orders,
    bool? isLoading,
    String? error,
    String? message,
    bool clearMessage = false,
    bool clearError = false,
  }) {
    return OrdersListState(
      orders: orders ?? this.orders,
      isLoading: isLoading ?? this.isLoading,
      error: clearError ? null : (error ?? this.error),
      message: clearMessage ? null : (message ?? this.message),
    );
  }
}

/// Semua order available (ride + food + send) untuk list screen.
final availableOrdersProvider =
    StateNotifierProvider<AvailableOrdersNotifier, OrdersListState>(
        (ref) => AvailableOrdersNotifier(ref.watch(orderServiceProvider)));

class AvailableOrdersNotifier extends StateNotifier<OrdersListState> {
  AvailableOrdersNotifier(this.service) : super(const OrdersListState());

  final OrderService service;

  Future<void> fetch() async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final orders = await service.fetchAvailableOrders();
      state = OrdersListState(orders: orders);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: '$e', clearError: false);
    }
  }

  void remove(String orderId) {
    state = state.copyWith(
      orders: state.orders.where((o) => o.id != orderId).toList(),
    );
  }
}

/// Order aktif driver + capacity check (pegas maks 3).
final activeOrdersProvider =
    StateNotifierProvider<ActiveOrdersNotifier, OrdersListState>(
        (ref) => ActiveOrdersNotifier(ref.watch(orderServiceProvider)));

class ActiveOrdersNotifier extends StateNotifier<OrdersListState> {
  ActiveOrdersNotifier(this.service) : super(const OrdersListState());

  final OrderService service;
  String? _driverId;

  Future<void> fetch() async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final driverId = _driverId;
      if (driverId == null || driverId.isEmpty) {
        state = OrdersListState(orders: const []);
        return;
      }
      final orders = await service.fetchActiveOrders(driverId);
      state = OrdersListState(orders: orders);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: '$e', clearError: false);
    }
  }

  void setDriverId(String? driverId) {
    if (driverId == _driverId) return;
    _driverId = driverId;
    if (driverId != null && driverId.isNotEmpty) fetch();
  }

  /// Accept order apapun (ride/send pakai endpoint accept, food lokal).
  /// Tolak jika kapasitas penuh (ROADMAP 3.10 C: >3 tolak).
  Future<bool> accept(DriverOrder order) async {
    if (state.isAtCapacity) {
      state = state.copyWith(
        error: 'Kapasitas penuh (maks $kMaxActiveOrders order aktif)',
        clearError: false,
      );
      return false;
    }
    try {
      switch (order.type) {
        case OrderType.ride:
          await service.acceptRide(order.id);
        case OrderType.send:
          await service.acceptSend(order.id);
        case OrderType.food:
          await service.acceptFood(order.id);
      }
      final accepted = order.copyWith(
        status: switch (order.type) {
          OrderType.ride || OrderType.send => 'DRIVER_ASSIGNED',
          OrderType.food => 'READY_FOR_PICKUP',
        },
      );
      state = state.copyWith(
        orders: [...state.orders, accepted],
        message: 'Order ${order.type.label} diterima',
      );
      return true;
    } catch (e) {
      state = state.copyWith(
        error: 'Gagal accept order: $e',
        clearError: false,
      );
      return false;
    }
  }

  /// Update status order (ride: DRIVER_ARRIVED/TRIP_STARTED/COMPLETED;
  /// food/send: PICKED_UP/IN_TRANSIT/DELIVERED).
  Future<bool> updateStatus(DriverOrder order, String status) async {
    try {
      switch (order.type) {
        case OrderType.ride:
          await service.updateRideStatus(order.id, status);
        case OrderType.food:
          await service.updateFoodStatus(order.id, status);
        case OrderType.send:
          await service.updateSendStatus(order.id, status);
      }
      _upsert(order.copyWith(status: status));
      return true;
    } catch (e) {
      state = state.copyWith(
        error: 'Gagal update status: $e',
        clearError: false,
      );
      return false;
    }
  }

  /// Mark satu stop send sebagai COMPLETED (PATCH .../stops/:stop_id).
  Future<bool> markStopDelivered(
    DriverOrder order,
    String stopId, {
    String? deliveryPhotoUrl,
  }) async {
    try {
      await service.updateSendStop(
        order.id,
        stopId,
        deliveryPhotoUrl: deliveryPhotoUrl,
      );
      final stops = order.stops
          .map((s) => s.id == stopId
              ? DriverStopCopier.copy(s, status: 'COMPLETED', deliveryPhotoUrl: deliveryPhotoUrl)
              : s)
          .toList();
      final allDone = stops.every((s) => s.isDelivered);
      _upsert(order.copyWith(
        stops: stops,
        status: allDone ? 'DELIVERED' : order.status,
      ));
      return true;
    } catch (e) {
      state = state.copyWith(
        error: 'Gagal update stop: $e',
        clearError: false,
      );
      return false;
    }
  }

  void _upsert(DriverOrder updated) {
    final next = state.orders
        .map((o) => o.id == updated.id ? updated : o)
        .toList();
    state = state.copyWith(orders: next);
  }
}

/// Helper copy untuk DriverStop (model immutable tanpa copyWith).
class DriverStopCopier {
  const DriverStopCopier._();

  static DriverStop copy(
    DriverStop s, {
    String? status,
    String? deliveryPhotoUrl,
  }) {
    return DriverStop(
      id: s.id,
      orderId: s.orderId,
      stopNumber: s.stopNumber,
      recipientName: s.recipientName,
      recipientPhone: s.recipientPhone,
      address: s.address,
      latitude: s.latitude,
      longitude: s.longitude,
      distanceKm: s.distanceKm,
      allocatedFare: s.allocatedFare,
      status: status ?? s.status,
      deliveryPhotoUrl: deliveryPhotoUrl ?? s.deliveryPhotoUrl,
    );
  }
}