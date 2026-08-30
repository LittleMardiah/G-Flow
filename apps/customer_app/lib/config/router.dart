import 'package:flutter/material.dart';

import '../screens/auth/login_screen.dart';
import '../screens/auth/register_screen.dart';
import '../screens/ride/ride_booking_screen.dart';
import '../screens/ride/ride_tracking_screen.dart';

/// Konstanta nama route. Parameter order (misal orderId) diteruskan via
/// `arguments: RideTrackingArgs` pada Navigator.pushNamed (bukan path
/// parameter), lihat `ride_tracking_screen.dart`.
class AppRoutes {
  AppRoutes._();

  static const String login = '/login';
  static const String register = '/register';
  static const String home = '/home';
  static const String rideTracking = '/ride-tracking';
}

/// Daftar route terpusat pengganti map inline di main.dart agar mudah
/// di-share dan di-test.
class AppRouter {
  AppRouter._();

  static Map<String, WidgetBuilder> get routes => {
        AppRoutes.login: (_) => const LoginScreen(),
        AppRoutes.register: (_) => const RegisterScreen(),
        AppRoutes.home: (_) => const RideBookingScreen(),
        AppRoutes.rideTracking: (_) => const RideTrackingScreen(),
      };
}