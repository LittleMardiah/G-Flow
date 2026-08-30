import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../models/driver_order.dart';
import '../providers/auth_provider.dart';
import '../screens/auth/login_screen.dart';
import '../screens/auth/register_screen.dart';
import '../screens/active_order_detail_screen.dart';
import '../screens/available_orders_screen.dart';
import '../screens/dashboard_screen.dart';
import '../screens/earnings_screen.dart';
import '../screens/multi_stop_delivery_screen.dart';

class AppRoutes {
  AppRoutes._();

  static const String login = '/login';
  static const String register = '/register';
  static const String dashboard = '/dashboard';
  static const String availableOrders = '/available-orders';
  static const String earnings = '/earnings';

  static String orderDetail(String orderId) => '/order/$orderId';
  static String multiStop(String orderId) => '/order/$orderId/multi-stop';
}

final goRouterProvider = Provider<GoRouter>((ref) {
  final authChanges = ValueNotifier(0);
  ref.listen<AuthState>(authProvider, (_, _) => authChanges.value++);
  final router = GoRouter(
    initialLocation: AppRoutes.login,
    refreshListenable: authChanges,
    redirect: (context, state) {
      final authed = ref.read(authProvider).isAuthenticated;
      final loc = state.matchedLocation;
      final isAuthScreen = loc == AppRoutes.login || loc == AppRoutes.register;
      if (!authed) return isAuthScreen ? null : AppRoutes.login;
      if (authed && isAuthScreen) return AppRoutes.dashboard;
      return null;
    },
    routes: [
      GoRoute(path: AppRoutes.login, name: 'login', builder: (context, state) => const LoginScreen()),
      GoRoute(path: AppRoutes.register, name: 'register', builder: (context, state) => const RegisterScreen()),
      GoRoute(path: AppRoutes.dashboard, name: 'dashboard', builder: (context, state) => const DashboardScreen()),
      GoRoute(
        path: AppRoutes.availableOrders,
        name: 'available-orders',
        builder: (context, state) => const AvailableOrdersScreen(),
      ),
      GoRoute(
        path: '/order/:orderId',
        name: 'order-detail',
        builder: (context, state) {
          final order = state.extra is DriverOrder ? state.extra as DriverOrder : null;
          return ActiveOrderDetailScreen(
            orderId: state.pathParameters['orderId'] ?? '',
            initialOrder: order,
          );
        },
      ),
      GoRoute(
        path: '/order/:orderId/multi-stop',
        name: 'multi-stop',
        builder: (context, state) {
          final order = state.extra is DriverOrder ? state.extra as DriverOrder : null;
          return MultiStopDeliveryScreen(
            orderId: state.pathParameters['orderId'] ?? '',
            initialOrder: order,
          );
        },
      ),
      GoRoute(path: AppRoutes.earnings, name: 'earnings', builder: (context, state) => const EarningsScreen()),
    ],
  );
  ref.onDispose(() {
    authChanges.dispose();
    router.dispose();
  });
  return router;
});