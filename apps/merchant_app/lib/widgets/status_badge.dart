import 'package:flutter/material.dart';

const Map<String, (Color, Color)> _kStatusColors = {
  'WAITING': (Colors.orange, Color(0xFFFFF3E0)),
  'CREATED': (Colors.orange, Color(0xFFFFF3E0)),
  'CONFIRMED': (Colors.blue, Color(0xFFE3F2FD)),
  'PREPARING': (Colors.purple, Color(0xFFF3E5F5)),
  'READY': (Colors.green, Color(0xFFE8F5E9)),
  'READY_FOR_PICKUP': (Colors.green, Color(0xFFE8F5E9)),
  'PICKED_UP': (Colors.teal, Color(0xFFE0F2F1)),
  'IN_TRANSIT': (Colors.cyan, Color(0xFFE0F7FA)),
  'DELIVERED': (Colors.green, Color(0xFFE8F5E9)),
  'SETTLED': (Colors.blueGrey, Color(0xFFECEFF1)),
  'CANCELLED': (Colors.red, Color(0xFFFFEBEE)),
};

class StatusBadge extends StatelessWidget {
  const StatusBadge({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    final key = status.toUpperCase();
    final (color, background) = _kStatusColors[key] ?? (Colors.blueGrey, const Color(0xFFECEFF1));
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        key,
        style: TextStyle(
          color: color,
          fontSize: 12,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.3,
        ),
      ),
    );
  }
}