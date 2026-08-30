import 'package:flutter/material.dart';

class StatusBadge extends StatelessWidget {
  const StatusBadge({super.key, required this.status});

  final String status;

  static const Map<String, Color> _kColors = {
    'SEARCHING_DRIVER': Colors.blueGrey,
    'DRIVER_ASSIGNED': Colors.indigo,
    'DRIVER_ARRIVED': Colors.teal,
    'TRIP_STARTED': Colors.blue,
    'READY_FOR_PICKUP': Colors.orange,
    'PICKED_UP': Colors.deepOrange,
    'IN_TRANSIT': Colors.lightBlue,
    'COMPLETED': Colors.green,
    'DELIVERED': Colors.green,
    'SETTLED': Colors.teal,
    'CANCELLED': Colors.red,
  };

  @override
  Widget build(BuildContext context) {
    final color = _kColors[status.toUpperCase()] ?? Colors.grey;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        status.replaceAll('_', ' '),
        style: TextStyle(
          color: color,
          fontSize: 10,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}