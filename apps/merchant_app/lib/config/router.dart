import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/auth_provider.dart';
import '../screens/analytics_screen.dart';
import '../screens/dashboard_screen.dart';
import '../screens/item_editor_screen.dart';
import '../screens/login_screen.dart';
import '../screens/menu_management_screen.dart';
import '../screens/order_detail_screen.dart';
import '../screens/orders_list_screen.dart';
import '../screens/register_screen.dart';

class AppRoutes {
  AppRoutes._();

  static const String login = '/login';
  static const String register = '/register';
  static const String dashboard = '/dashboard';
  static const String orders = '/orders';
  static const String menuManagement = '/menu';
  static const String analytics = '/analytics';

  static String orderDetail(String orderId) => '/orders/$orderId';
  static String newItem(String menuId) => '/menu/$menuId/items/new';
  static String editItem(String menuId, String itemId) => '/menu/$menuId/items/$itemId';
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
      GoRoute(path: AppRoutes.orders, name: 'orders', builder: (context, state) => const OrdersListScreen()),
      GoRoute(
        path: '/orders/:orderId',
        name: 'order-detail',
        builder: (context, state) => OrderDetailScreen(orderId: state.pathParameters['orderId'] ?? ''),
      ),
      GoRoute(path: AppRoutes.menuManagement, name: 'menu', builder: (context, state) => const MenuManagementScreen()),
      GoRoute(
        path: '/menu/:menuId/items/new',
        name: 'item-new',
        builder: (context, state) => ItemEditorScreen(initialMenuId: state.pathParameters['menuId']),
      ),
      GoRoute(
        path: '/menu/:menuId/items/:itemId',
        name: 'item-edit',
        builder: (context, state) => ItemEditorScreen(
          itemId: state.pathParameters['itemId'],
          initialMenuId: state.pathParameters['menuId'],
        ),
      ),
      GoRoute(path: AppRoutes.analytics, name: 'analytics', builder: (context, state) => const AnalyticsScreen()),
    ],
  );
  ref.onDispose(() {
    authChanges.dispose();
    router.dispose();
  });
  return router;
});